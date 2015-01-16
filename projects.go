package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-redmine"
)

type ProjectsCommand struct{}

func (t ProjectsCommand) Keyword() string {
	return "projects"
}

func (c ProjectsCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (t ProjectsCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword() + " ",
		Subtitle:     "List the projects you're working on",
	}
}

func (t ProjectsCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item

	parts := alfred.SplitAndTrimQuery(query)
	if len(parts) > 1 {
		// user has specified a project name

		projName := parts[0]
		pi := indexOfByName(RmProjectList(cache.Projects), projName)
		if pi == -1 {
			return []alfred.Item{}, fmt.Errorf("Invalid project %d", projName)
		}

		project := cache.Projects[pi]

		var issues []redmine.Issue
		for _, issue := range cache.Issues {
			if issue.Project.Id == project.Id {
				issues = append(issues, issue)
			}
		}

		prefix := prefix + parts[0] + alfred.Separator + " "
		items = append(items, createIssueItems(prefix, parts[1], issues)...)
	} else {
		// user has specified a partial project name or no name
		log.Println("Listing all user projects")

		// first, filter all projects based on my open issues
		projects := map[int]*redmine.Issue{}
		activeProjects := map[int]bool{}
		issueCounts := map[int]int{}
		closed := getClosedStatusIds()

		for i := range cache.Issues {
			issue := &cache.Issues[i]
			if _, ok := closed[issue.Status.Id]; !ok {
				// projects with at least one non-closed issue are "active"
				activeProjects[issue.Project.Id] = true
			}

			existing, ok := projects[issue.Project.Id]
			if !ok || dueDateIsBefore(issue, existing) {
				// store the project with the soonest due date in the projects list
				projects[issue.Project.Id] = issue
			}

			issueCounts[issue.Project.Id]++
		}

		// First, add the projects with active issues
		for _, project := range cache.Projects {
			if _, ok := activeProjects[project.Id]; !ok {
				// skip inactive projects
				continue
			}

			p, ok := projects[project.Id]
			if ok && alfred.FuzzyMatches(project.Name, query) {
				subTitle := fmt.Sprintf("%d issues", issueCounts[project.Id])
				if p.DueDate != "" {
					dueDate, _ := time.Parse("2006-01-02", p.DueDate)
					subTitle += ", first is due " + toHumanDateString(dueDate)
				}

				items = append(items, alfred.Item{
					Title:        project.Name,
					Autocomplete: prefix + project.Name + alfred.Separator + " ",
					Subtitle:     subTitle,
					Arg:          "open " + fmt.Sprintf("%s/projects/%v/issues", config.RedmineUrl, project.Id)})
			}
		}

		// Next, add the projects that recently had active issues
		for _, project := range cache.Projects {
			if _, ok := activeProjects[project.Id]; ok {
				// skip active projects
				continue
			}

			_, ok := projects[project.Id]
			if ok && alfred.FuzzyMatches(project.Name, query) {
				items = append(items, alfred.Item{
					Title:        project.Name,
					Autocomplete: prefix + project.Name + alfred.Separator + " ",
					Arg:          "open " + fmt.Sprintf("%s/projects/%v/issues", config.RedmineUrl, project.Id)})
			}
		}
	}

	return items, nil
}
