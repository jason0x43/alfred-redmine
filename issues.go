package main

import (
	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-redmine"
)

type IssuesCommand struct{}

func (t IssuesCommand) Keyword() string {
	return "issues"
}

func (c IssuesCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (t IssuesCommand) MenuItem() alfred.Item {
	issuesUrl := config.RedmineUrl + "/issues?utf8=✓&set_filter=1&" +
		"f[]=assigned_to_id&op[assigned_to_id]==&v[assigned_to_id][]=me&" +
		"f[]=status_id&op[status_id]=o&f[]=&c[]=project&c[]=status&c[]=priority&c[]=subject&" +
		"c[]=updated_on&c[]=due_date&c[]=estimated_hours&c[]=spent_hours&c[]=done_ratio&group_by="

	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword() + " ",
		Arg:          "open " + issuesUrl,
		Subtitle:     "List your assigned issues",
	}
}

func (t IssuesCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	var issues []redmine.Issue
	closed := getClosedStatusIds()

	for _, issue := range cache.Issues {
		if _, ok := closed[issue.Status.Id]; ok {
			continue
		}

		match := issue.Project.Name + " " + issue.Subject + " " + issue.Project.Name

		if alfred.FuzzyMatches(match, query) {
			issues = append(issues, issue)
		}
	}

	items = append(items, createIssueItems(prefix, query, issues)...)

	if query == "" {
		issuesUrl := config.RedmineUrl + "/issues?utf8=✓&set_filter=1&" +
			"f[]=assigned_to_id&op[assigned_to_id]==&v[assigned_to_id][]=me&" +
			"f[]=status_id&op[status_id]=o&f[]=&c[]=project&c[]=status&c[]=priority&c[]=subject&" +
			"c[]=updated_on&c[]=due_date&c[]=estimated_hours&c[]=spent_hours&c[]=done_ratio&group_by="

		item := alfred.Item{
			Title:    "View all on Redmine",
			Subtitle: alfred.Line,
			Arg:      "open " + issuesUrl,
		}
		items = alfred.InsertItem(items, item, 0)
	}

	return items, nil
}
