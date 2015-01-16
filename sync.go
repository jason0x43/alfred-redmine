package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-redmine"
)

type SyncCommand struct{}

func (t SyncCommand) Keyword() string {
	return "sync"
}

func (c SyncCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (t SyncCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword(),
		Valid:        alfred.Invalid,
		Subtitle:     "Sync with your Redmine server",
	}
}

func (t SyncCommand) Items(prefix, query string) ([]alfred.Item, error) {
	err := refresh()
	if err != nil {
		return []alfred.Item{}, err
	}

	return []alfred.Item{alfred.Item{Title: "Synchronized!"}}, nil
}

func createIssueItems(prefix, query string, issues []redmine.Issue) []alfred.Item {
	sort.Sort(byDueDate(issues))
	sort.Stable(sort.Reverse(byPriority(issues)))
	sort.Stable(byAssignment(issues))

	var items []alfred.Item

	for _, issue := range issues {
		if alfred.FuzzyMatches(issue.Subject, query) {
			subTitle := issue.Project.Name

			if issue.DueDate != "" {
				dueDate, _ := time.Parse("2006-01-02", issue.DueDate)
				subTitle += ", due " + toHumanDateString(dueDate)
			}
			subTitle += ", " + issue.Priority.Name

			item := alfred.Item{
				Title:        issue.Subject,
				Subtitle:     subTitle,
				Autocomplete: prefix + issue.Subject,
				Arg:          "open " + fmt.Sprintf("%s/issues/%v", config.RedmineUrl, issue.Id),
			}

			if issue.AssignedTo.Id == cache.User.Id {
				item.Icon = "icon_me.png"
			}

			items = append(items, item)
		}
	}

	return items
}
