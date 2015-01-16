package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-redmine"
)

type TimesheetCommand struct{}

func (t TimesheetCommand) Keyword() string {
	return "timesheet"
}

func (c TimesheetCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (t TimesheetCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword() + " ",
		Subtitle:     "Generate a timesheet",
	}
}

func (t TimesheetCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	var since string
	var until string
	var span string

	parts := alfred.SplitQuery(query)

	for _, value := range []string{"today", "yesterday", "this week"} {
		if alfred.FuzzyMatches(value, parts[0]) {
			switch value {
			case "today":
				since = toIsoDateString(time.Now())
				until = since
			case "yesterday":
				since = toIsoDateString(time.Now().AddDate(0, 0, -1))
				until = since
			case "this week":
				start := time.Now()
				if start.Weekday() > 0 {
					delta := 1 - int(start.Weekday())
					since = toIsoDateString(start.AddDate(0, 0, delta))
					until = toIsoDateString(time.Now())
				}
			}

			items = append(items, alfred.Item{
				Autocomplete: prefix + value + alfred.Separator + " ",
				Title:        value,
				Arg:          "open " + getTimesheetUrl(since, until),
				Subtitle:     "Generate a report for " + value,
			})
		}

		if value == parts[0] {
			span = parts[0]
		}
	}

	if span == "" {
		return items, nil
	} else if len(items) == 0 {
		items = append(items, alfred.Item{
			Valid: alfred.Invalid,
			Title: "Enter a valid range",
		})
		return items, nil
	}

	var err error

	if since != "" && until != "" {
		var query string
		if len(parts) > 1 {
			query = strings.Join(parts[1:], alfred.Separator+" ")
		}

		items, err = createTimesheetItems(prefix+span, query, span, since, until)
		if err != nil {
			return []alfred.Item{}, err
		}
	}

	if len(items) == 0 {
		items = append(items, alfred.Item{
			Title: "No entries",
			Valid: alfred.Invalid,
		})
	}

	return items, nil
}

// support -------------------------------------------------------------

type timesheetIssue struct {
	total float64
	name  string
	issue *redmine.Issue
}

type timesheetProject struct {
	total   float64
	name    string
	id      int
	project *redmine.Project
	issues  []*timesheetIssue
}

type timesheet struct {
	total    float64
	projects []*timesheetProject
}

func generateTimesheet(since, until string) (timesheet, error) {
	log.Printf("generating timesheet from %s to %s", since, until)
	timesheet := timesheet{projects: []*timesheetProject{}}
	projects := map[int]*timesheetProject{}
	issues := map[int]*timesheetIssue{}

	err := getMissingIssues()
	if err != nil {
		return timesheet, err
	}

	for i := range cache.TimeEntries {
		entry := &cache.TimeEntries[i]

		if entry.SpentOn >= since && entry.SpentOn <= until {
			var project *timesheetProject
			var issue *timesheetIssue
			var ok bool

			if project, ok = projects[entry.Project.Id]; !ok {
				project = &timesheetProject{
					name:   entry.Project.Name,
					id:     entry.Project.Id,
					issues: []*timesheetIssue{},
				}
				projects[entry.Project.Id] = project
				timesheet.projects = append(timesheet.projects, project)

				idx := indexOfById(RmProjectList(cache.Projects), entry.Project.Id)
				if idx == -1 {
					log.Printf("Missing project %v", entry.Project.Id)
				}
				project.project = &cache.Projects[idx]
			}

			if issue, ok = issues[entry.Issue.Id]; !ok {
				idx := indexOfById(IssueList(cache.Issues), entry.Issue.Id)
				ri := cache.Issues[idx]
				issue = &timesheetIssue{
					name:  ri.Subject,
					issue: &ri,
				}
				issues[entry.Issue.Id] = issue
				project.issues = append(project.issues, issue)
			}

			issue.total += entry.Hours
			project.total += entry.Hours
			timesheet.total += entry.Hours
		}
	}

	return timesheet, nil
}

func getMissingIssues() error {
	issueChan := make(chan redmine.Issue)
	errorChan := make(chan error)

	var ids []int
	for _, issue := range cache.Issues {
		ids = append(ids, issue.Id)
	}

	var toGet []int
	for _, entry := range cache.TimeEntries {
		found := false
		for _, id := range ids {
			if entry.Issue.Id == id {
				found = true
				break
			}
		}
		if !found {
			toGet = append(toGet, entry.Issue.Id)
		}
	}

	session := redmine.OpenSession(config.RedmineUrl, config.ApiKey)

	for _, id := range toGet {
		go func(id int) {
			ri, err := session.GetIssue(id)
			if err != nil {
				errorChan <- err
			} else {
				issueChan <- ri
			}
		}(id)
	}

	for _ = range toGet {
		select {
		case issue := <-issueChan:
			cache.Issues = append(cache.Issues, issue)
			log.Println("appended issue", issue.Id)
		case err := <-errorChan:
			return err
		}
	}

	err := alfred.SaveJson(cacheFile, &cache)
	if err != nil {
		log.Println("Error saving cache:", err)
	}

	return nil
}

func createTimesheetItems(prefix, query, span, since, until string) ([]alfred.Item, error) {
	var items []alfred.Item
	parts := alfred.SplitAndTrimQuery(query)

	timesheet, err := generateTimesheet(since, until)
	if err != nil {
		return items, err
	}

	if len(timesheet.projects) > 0 {
		total := 0.0
		totalName := ""

		if len(parts) > 1 {
			// we have a project name terminator, so list the project's time entries

			name := parts[0]
			proj := indexOfByName(ProjectList(timesheet.projects), name)
			if proj == -1 {
				return items, fmt.Errorf("Couldn't find project '%s'", name)
			}

			project := timesheet.projects[proj]
			issueQuery := parts[1]

			session := redmine.OpenSession(config.RedmineUrl, config.ApiKey)

			for _, issue := range project.issues {
				if alfred.FuzzyMatches(issue.name, issueQuery) {
					items = append(items, alfred.Item{
						Autocomplete: prefix + alfred.Separator + " " + project.name + alfred.Separator + " " + issue.name,
						Title:        issue.name,
						Arg:          "open|" + session.IssueUrl(*issue.issue),
						Subtitle:     fmt.Sprintf("%.2f", issue.total)})
				}
			}

			total = project.total

			if issueQuery == "" {
				totalName = span + " for " + project.name
			}
		} else {
			// no project name terminator, so filter projects by name

			projectQuery := parts[0]

			for _, project := range timesheet.projects {
				entryTitle := project.name

				item := alfred.Item{
					Valid:        alfred.Invalid,
					Autocomplete: prefix + alfred.Separator + " " + entryTitle + alfred.Separator + " ",
					Title:        entryTitle,
					Subtitle:     fmt.Sprintf("%.2f", project.total)}

				if projectQuery != "" {
					if alfred.FuzzyMatches(entryTitle, projectQuery) {
						items = append(items, item)
						total += project.total
					}
				} else {
					items = append(items, item)
					total += project.total
				}
			}

			if projectQuery == "" {
				totalName = span
			}
		}

		sort.Sort(alfred.ByTitle(items))

		if totalName != "" {
			item := alfred.Item{
				Title:        fmt.Sprintf("Total hours %s: %.2f", totalName, total),
				Arg:          "open " + getTimesheetUrl(since, until),
				Autocomplete: query,
				Subtitle:     alfred.Line,
			}
			items = alfred.InsertItem(items, item, 0)
		}
	}

	return items, nil
}

func getTimesheetUrl(since, until string) string {
	addrFormat := config.RedmineUrl + "/timesheet/report" +
		"?timesheet[date_from]=%v" +
		"&timesheet[date_to]=%v" +
		"&timesheet[sort]=project" +
		"&timesheet[users][]=%v"

	return fmt.Sprintf(addrFormat, since, until, cache.User.Id)
}
