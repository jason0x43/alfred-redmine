package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/jason0x43/go-alfred"
)

// ProjectsCommand is a command
type ProjectsCommand struct{}

// About returns information about a command
func (c ProjectsCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     projectsKeyword,
		Description: "List the projects you're working on",
		IsEnabled:   config.APIKey != "",
	}
}

// Items returns a list of filter items
func (c ProjectsCommand) Items(arg, data string) (items []alfred.Item, err error) {
	var cfg projectCfg
	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Invalid issue config")
		}
	}

	if err = checkRefresh(); err != nil {
		return
	}

	// first, filter all projects based on user's open issues
	projects := map[int]Issue{}
	activeProjects := map[int]bool{}
	issueCounts := map[int]int{}
	closed := getClosedStatusIDs()

	for i := range cache.Issues {
		issue := cache.Issues[i]
		if _, ok := closed[issue.Status.ID]; !ok {
			// projects with at least one non-closed issue are "active"
			activeProjects[issue.Project.ID] = true
		}

		if existing, ok := projects[issue.Project.ID]; !ok || dueDateIsBefore(issue.DueDate, existing.DueDate) {
			// store the project with the soonest due date in the projects list
			projects[issue.Project.ID] = issue
		}

		issueCounts[issue.Project.ID]++
	}

	// First, add the projects with active issues
	for _, project := range cache.Projects {
		if _, ok := activeProjects[project.ID]; !ok {
			// skip inactive projects
			continue
		}

		if p, ok := projects[project.ID]; ok && alfred.FuzzyMatches(project.Name, arg) {
			subTitle := fmt.Sprintf("%d issues", issueCounts[project.ID])
			if p.DueDate != "" {
				dueDate, _ := time.Parse("2006-01-02", p.DueDate)
				subTitle += ", first is due " + toHumanDateString(dueDate)
			}

			item := alfred.Item{
				UID:          fmt.Sprintf("redmineproject-%d", project.ID),
				Title:        project.Name,
				Autocomplete: project.Name,
				Subtitle:     subTitle,
				Arg: &alfred.ItemArg{
					Keyword: issuesKeyword,
					Data:    alfred.Stringify(&issueCfg{ProjectID: &project.ID}),
				},
			}

			item.AddMod(alfred.ModAlt, alfred.ItemMod{
				Subtitle: "Open this project in Redmine",
				Arg: &alfred.ItemArg{
					Keyword: projectsKeyword,
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(&projectCfg{ToOpen: getProjectURL(project.ID)}),
				},
			})

			items = append(items, item)
		}
	}

	// Next, add the projects that recently had active issues
	for _, project := range cache.Projects {
		if _, ok := activeProjects[project.ID]; ok {
			// skip active projects
			continue
		}

		if _, ok := projects[project.ID]; ok && alfred.FuzzyMatches(project.Name, arg) {
			item := alfred.Item{
				UID:          fmt.Sprintf("redmineproject-%d", project.ID),
				Title:        project.Name,
				Autocomplete: project.Name,
				Arg: &alfred.ItemArg{
					Keyword: issuesKeyword,
					Data:    alfred.Stringify(&issueCfg{ProjectID: &project.ID}),
				},
			}

			item.AddMod(alfred.ModAlt, alfred.ItemMod{
				Subtitle: "Open this project in Redmine",
				Arg: &alfred.ItemArg{
					Keyword: projectsKeyword,
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(&projectCfg{ToOpen: getProjectURL(project.ID)}),
				},
			})

			items = append(items, item)
		}
	}

	return
}

// Do runs the command
func (c ProjectsCommand) Do(data string) (out string, err error) {
	var cfg projectCfg
	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			return "", fmt.Errorf("Invalid project config")
		}
	}

	if cfg.ToOpen != "" {
		err = exec.Command("open", cfg.ToOpen).Run()
	}

	return
}

// support -------------------------------------------------------------------

const projectsKeyword = "projects"

type projectCfg struct {
	ProjectID *int
	ToOpen    string
}

func getProjectURL(pid int) string {
	// return fmt.Sprintf("%s/projects/%v/issues", config.RedmineURL, pid)
	return fmt.Sprintf("%s/projects/%v", config.RedmineURL, pid)
}
