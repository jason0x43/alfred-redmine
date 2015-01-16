package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

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

func (t IssuesCommand) Items(prefix, query string) (items []alfred.Item, err error) {
	parts := alfred.SplitAndTrimQuery(query)
	log.Printf("parts: %s", parts)

	if len(parts) > 1 {
		var issue redmine.Issue
		if issue, err = getIssueByStringId(parts[0]); err != nil {
			return
		}

		prefix += strconv.Itoa(issue.Id) + alfred.Separator + " "

		if alfred.FuzzyMatches("subject", parts[1]) {
			items = append(items, alfred.Item{
				Title: "subject: " + issue.Subject,
				Valid: alfred.Invalid,
			})
		}
		if alfred.FuzzyMatches("status", parts[1]) {
			if parts[1] == "status" {
				for _, st := range cache.IssueStatuses {
					newIssue := redmine.UpdateIssue{Status: st.Id}
					data := updateIssueMessage{Action: "update", Id: issue.Id, Issue: newIssue}
					dataString, _ := json.Marshal(data)

					items = append(items, alfred.Item{
						Title: st.Name,
						Arg:   t.Keyword() + " " + string(dataString),
					})
				}
			} else {
				items = append(items, alfred.Item{
					Title:        "status: " + issue.Status.Name,
					Autocomplete: prefix + "status",
					Valid:        alfred.Invalid,
				})
			}
		}
	} else {
		var issues []redmine.Issue
		closed := getClosedStatusIds()

		for _, issue := range cache.Issues {
			if _, isClosed := closed[issue.Status.Id]; !isClosed {
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
	}

	return
}

func (t IssuesCommand) Do(query string) (out string, err error) {
	log.Printf("issues %s", query)

	var data updateIssueMessage
	if err = json.Unmarshal([]byte(query), &data); err != nil {
		return
	}

	session := redmine.OpenSession(config.RedmineUrl, config.ApiKey)

	switch data.Action {
	case "update":
		if err = session.UpdateIssue(data.Id, data.Issue); err != nil {
			return
		}

		var issue redmine.Issue
		if issue, err = session.GetIssue(data.Id); err != nil {
			return
		}

		for i, _ := range cache.Issues {
			if cache.Issues[i].Id == issue.Id {
				cache.Issues[i] = issue
				if err := alfred.SaveJson(cacheFile, &cache); err != nil {
					log.Printf("Error saving cache: %v\n", err)
				}
				break
			}
		}
	}

	return fmt.Sprintf("Updated issue %d", data.Id), nil
}

// support -------------------------------------------------------------

type updateIssueMessage struct {
	Action string
	Id     int
	Issue  redmine.UpdateIssue
}

func createIssueItems(prefix, query string, issues []redmine.Issue) []alfred.Item {
	sort.Sort(byDueDate(issues))
	sort.Stable(sort.Reverse(byPriority(issues)))
	sort.Stable(byAssignment(issues))

	var items []alfred.Item

	for _, issue := range issues {
		log.Printf("checking if '" + query + "' fuzzy matches '" + issue.Subject + "'")
		if alfred.FuzzyMatches(issue.Subject, query) || alfred.FuzzyMatches(strconv.Itoa(issue.Id), query) {
			items = append(items, createIssueItem(prefix, issue))
		}
	}

	return items
}

func createIssueItem(prefix string, issue redmine.Issue) (item alfred.Item) {
	subTitle := fmt.Sprintf("%d [%s]", issue.Id, issue.Project.Name)
	if issue.DueDate != "" {
		dueDate, _ := time.Parse("2006-01-02", issue.DueDate)
		subTitle += " Due " + toHumanDateString(dueDate) + ","
	}
	subTitle += " " + issue.Priority.Name

	item.Title = issue.Subject
	item.Subtitle = subTitle
	item.Autocomplete = prefix + strconv.Itoa(issue.Id) + alfred.Separator + " "
	item.Arg = "open " + fmt.Sprintf("%s/issues/%v", config.RedmineUrl, issue.Id)

	if issue.AssignedTo.Id == cache.User.Id {
		item.Icon = "icon_me.png"
	}

	return
}

func getIssueByStringId(idStr string) (issue redmine.Issue, err error) {
	var id int
	if id, err = strconv.Atoi(idStr); err != nil {
		return
	}

	for _, i := range cache.Issues {
		if i.Id == id {
			issue = i
			return
		}
	}

	return issue, errors.New("Invalid ID " + idStr)
}
