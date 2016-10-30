package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
)

// TimesheetCommand is a command
type TimesheetCommand struct{}

// About returns information about a command
func (c TimesheetCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     timesheetKeyword,
		Description: "Generate a timesheet",
		IsEnabled:   config.APIKey != "" && cache.User.ID != 0,
	}
}

// Items returns a list of filter items
func (c TimesheetCommand) Items(arg, data string) (items []alfred.Item, err error) {
	var cfg timesheetCfg
	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Invalid timesheet config")
		}
	}

	var span span

	if cfg.Span != nil {
		span = *cfg.Span
		if items, err = createTimesheetItems(arg, span); err != nil {
			return
		}
	} else {
		for _, value := range []string{"today", "yesterday", "week"} {
			if alfred.FuzzyMatches(value, arg) {
				span, _ := getSpan(value)
				items = append(items, createTimesheetItem(span))
			}
		}

		if matched, _ := regexp.MatchString(`^\d`, arg); matched {
			if span, err = getSpan(arg); err == nil {
				items = append(items, createTimesheetItem(span))
			}
		}
	}

	if len(items) == 0 {
		items = append(items, alfred.Item{
			Title: "No entries",
		})
	}

	return
}

// Do runs the command
func (c TimesheetCommand) Do(data string) (out string, err error) {
	var cfg timesheetCfg
	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			return "", fmt.Errorf("Invalid issue config")
		}
	}

	if cfg.ToOpen != "" {
		err = exec.Command("open", cfg.ToOpen).Run()
	}

	return
}

// support -------------------------------------------------------------

const timesheetKeyword = "timesheet"

type timesheetCfg struct {
	ToOpen string
	Span   *span
}

type timesheetIssue struct {
	total float64
	name  string
	issue *Issue
}

type timesheetProject struct {
	total   float64
	name    string
	id      int
	project *Project
	issues  []*timesheetIssue
}

type timesheet struct {
	total    float64
	projects []*timesheetProject
}

type span struct {
	Name  string
	Label string
	From  string
	To    string
}

func createTimesheetItem(span span) (item alfred.Item) {
	label := span.Name
	if span.Label != "" {
		label = span.Label
	}

	item = alfred.Item{
		Autocomplete: label,
		Title:        label,
		Subtitle:     "Generate a timesheet for " + label,
		Arg: &alfred.ItemArg{
			Keyword: timesheetKeyword,
			Data:    alfred.Stringify(&timesheetCfg{Span: &span}),
		},
	}

	item.AddMod(alfred.ModCmd, alfred.ItemMod{
		Subtitle: "Open the timesheet for " + sublabel,
		Arg: &alfred.ItemArg{
			Keyword: timesheetKeyword,
			Mode:    alfred.ModeDo,
			Data:    alfred.Stringify(&timesheetCfg{ToOpen: getTimesheetURL(span)}),
		},
	})

	return
}

func generateTimesheet(span span) (timesheet timesheet, err error) {
	log.Printf("generating timesheet from %s to %s", span.From, span.To)

	projects := map[int]*timesheetProject{}
	issues := map[int]*timesheetIssue{}

	if err = getMissingIssues(); err != nil {
		return
	}

	for i := range cache.TimeEntries {
		entry := &cache.TimeEntries[i]

		if entry.SpentOn >= span.From && entry.SpentOn <= span.To {
			var project *timesheetProject
			var issue *timesheetIssue
			var ok bool

			if project, ok = projects[entry.Project.ID]; !ok {
				project = &timesheetProject{
					name:   entry.Project.Name,
					id:     entry.Project.ID,
					issues: []*timesheetIssue{},
				}

				if idx := indexOfByID(projectList(cache.Projects), entry.Project.ID); idx == -1 {
					log.Printf("Missing project %v", entry.Project.ID)
				} else {
					project.project = &cache.Projects[idx]
				}

				projects[entry.Project.ID] = project
				timesheet.projects = append(timesheet.projects, project)
			}

			if issue, ok = issues[entry.Issue.ID]; !ok {
				idx := indexOfByID(issueList(cache.Issues), entry.Issue.ID)
				ri := cache.Issues[idx]
				issue = &timesheetIssue{
					name:  ri.Subject,
					issue: &ri,
				}
				issues[entry.Issue.ID] = issue
				project.issues = append(project.issues, issue)
			}

			issue.total += entry.Hours
			project.total += entry.Hours
			timesheet.total += entry.Hours
		}
	}

	return
}

var dateFormats = map[string]*regexp.Regexp{
	"1/2":      regexp.MustCompile(`^\d\d?\/\d\d?$`),
	"1/2/06":   regexp.MustCompile(`^\d\d?\/\d\d?\/\d\d$`),
	"1/2/2006": regexp.MustCompile(`^\d\d?\/\d\d?\/\d\d\d\d$`),
	"2006-1-2": regexp.MustCompile(`^\d\d\d\d-\d\d?-\d\d$`),
}

// return true if the string can be parsed as a date
func getDateLayout(s string) string {
	for layout, matcher := range dateFormats {
		if matcher.MatchString(s) {
			return layout
		}
	}
	return ""
}

func getMissingIssues() error {
	issueChan := make(chan Issue)
	errorChan := make(chan error)

	var ids []int
	for _, issue := range cache.Issues {
		ids = append(ids, issue.ID)
	}

	var toGet []int
	for _, entry := range cache.TimeEntries {
		found := false
		for _, id := range ids {
			if entry.Issue.ID == id {
				found = true
				break
			}
		}
		if !found {
			toGet = append(toGet, entry.Issue.ID)
		}
	}

	session := OpenSession(config.RedmineURL, config.APIKey)

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
			log.Println("appended issue", issue.ID)
		case err := <-errorChan:
			return err
		}
	}

	err := alfred.SaveJSON(cacheFile, &cache)
	if err != nil {
		log.Println("Error saving cache:", err)
	}

	return nil
}

// expand fills in the start and end times for a span
func getSpan(arg string) (s span, err error) {
	if arg == "today" {
		today := time.Now().Format("2006-01-02")
		s.Name = arg
		s.From = today
		s.To = today
	} else if arg == "yesterday" {
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		s.Name = arg
		s.From = yesterday
		s.To = yesterday
	} else if arg == "week" {
		s.Name = "week"
		s.Label = "this week"
		delta := -int(time.Now().Weekday())
		start := time.Now()
		s.From = start.AddDate(0, 0, delta).Format("2006-01-02")
		s.To = time.Now().Format("2006-01-02")
	} else {
		if strings.Contains(arg, "..") {
			parts := alfred.CleanSplitN(arg, "..", 2)
			if len(parts) == 2 {
				var span1 span
				var span2 span
				if span1, err = getSpan(parts[0]); err == nil {
					if span2, err = getSpan(parts[1]); err == nil {
						s.Name = arg
						s.From = span1.From
						s.To = span2.To
					}
				}
			}
		} else {
			if layout := getDateLayout(arg); layout != "" {
				var from time.Time
				if from, err = time.Parse(layout, arg); err != nil {
					return
				}
				year := from.Year()
				if year == 0 {
					year = time.Now().Year()
				}
				s.Name = arg
				date := time.Date(year, from.Month(), from.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
				s.From = date
				s.To = date
				s.Name = arg
			}
		}
	}

	if err == nil && s.Name == "" {
		err = fmt.Errorf("Unable to parse span '%s'", arg)
	}

	return
}
func createTimesheetItems(arg string, span span) (items []alfred.Item, err error) {
	dlog.Printf("creating items for %#v", span)

	var timesheet timesheet
	if timesheet, err = generateTimesheet(span); err != nil {
		return
	}

	if len(timesheet.projects) > 0 {
		total := 0.0
		totalName := ""

		for _, project := range timesheet.projects {
			if arg == "" || alfred.FuzzyMatches(project.name, arg) {
				items = append(items, alfred.Item{
					Autocomplete: project.name,
					Title:        project.name,
					Subtitle:     fmt.Sprintf("%.2f", project.total),
				})
				total += project.total
			}
		}

		if arg == "" {
			totalName = span.Name
		}

		sort.Sort(alfred.ByTitle(items))

		if totalName != "" {
			item := alfred.Item{
				Title:        fmt.Sprintf("Total hours %s: %.2f", totalName, total),
				Autocomplete: arg,
				Subtitle:     alfred.Line,
				Arg: &alfred.ItemArg{
					Keyword: timesheetKeyword,
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(&timesheetCfg{ToOpen: getTimesheetURL(span)}),
				},
			}
			items = alfred.InsertItem(items, item, 0)
		}
	}

	return
}

func getTimesheetURL(span span) string {
	addrFormat := config.RedmineURL + "/timesheet/report" +
		"?timesheet[date_from]=%v" +
		"&timesheet[date_to]=%v" +
		"&timesheet[sort]=project" +
		"&timesheet[users][]=%v"

	return fmt.Sprintf(addrFormat, span.From, span.To, cache.User.ID)
}
