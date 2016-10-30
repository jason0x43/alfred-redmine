package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"github.com/jason0x43/go-alfred"
)

// IssuesCommand is a command
type IssuesCommand struct{}

// About returns information about a command
func (c IssuesCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     issuesKeyword,
		Description: "List your assigned issues",
		IsEnabled:   config.APIKey != "",
		Mods: map[alfred.ModKey]alfred.ItemMod{
			alfred.ModCmd: alfred.ItemMod{
				Arg: &alfred.ItemArg{
					Keyword: issuesKeyword,
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(&issueCfg{ToOpen: getMyIssuesURL()}),
				},
			},
		},
	}
}

// Items returns a list of filter items
func (c IssuesCommand) Items(arg, data string) (items []alfred.Item, err error) {
	var cfg issueCfg
	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Invalid issue config")
		}
	}

	if err = checkRefresh(); err != nil {
		return
	}

	pid := -1
	if cfg.ProjectID != nil {
		pid = *cfg.ProjectID
	}

	if cfg.IssueID != nil {
		var idx int
		if idx = indexOfByID(issueList(cache.Issues), *cfg.IssueID); idx == -1 {
			err = fmt.Errorf("Invalid issue ID %d", *cfg.IssueID)
			return
		}

		issue := cache.Issues[idx]

		if arg != "" {
			parts := alfred.CleanSplitN(arg, " ", 2)

			if alfred.FuzzyMatches("subject:", parts[0]) {
				items = append(items, alfred.Item{
					Title: "subject: " + issue.Subject,
				})
			}

			if alfred.FuzzyMatches("status:", parts[0]) {
				if parts[0] == "status:" {
					for _, st := range cache.IssueStatuses {
						items = append(items, alfred.Item{
							Title: st.Name,
							Arg: &alfred.ItemArg{
								Keyword: issuesKeyword,
								Mode:    alfred.ModeDo,
								Data: alfred.Stringify(&issueCfg{
									ToUpdate: &updateIssueMessage{
										Action: "update",
										ID:     issue.ID,
										Issue:  UpdateIssue{Status: st.ID},
									},
								}),
							},
						})
					}
				} else {
					items = append(items, alfred.Item{
						Title:        "status: " + issue.Status.Name,
						Autocomplete: "status: " + issue.Status.Name,
					})
				}
			}
		}
	} else {
		closed := getClosedStatusIDs()
		var issues []Issue

		for _, issue := range cache.Issues {
			if _, isClosed := closed[issue.Status.ID]; !isClosed {
				issues = append(issues, issue)
			}
		}

		items = append(items, createIssueItems(arg, pid, issues)...)

		if arg == "" {
			if pid == -1 {
				dlog.Printf("adding View All item")
				item := alfred.Item{
					Title:    "View all on Redmine",
					Subtitle: alfred.Line,
					Arg: &alfred.ItemArg{
						Keyword: issuesKeyword,
						Mode:    alfred.ModeDo,
						Data:    alfred.Stringify(&issueCfg{ToOpen: getMyIssuesURL()}),
					},
				}
				items = alfred.InsertItem(items, item, 0)
			} else {
				dlog.Printf("adding Project name item")
				pi := indexOfByID(projectList(cache.Projects), pid)
				project := cache.Projects[pi]
				item := alfred.Item{
					Title:    project.Name,
					Subtitle: alfred.Line,
					Arg:      &alfred.ItemArg{Keyword: projectsKeyword},
				}
				items = alfred.InsertItem(items, item, 0)
			}
		}
	}

	return
}

// Do runs the command
func (c IssuesCommand) Do(data string) (out string, err error) {
	var cfg issueCfg
	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			return "", fmt.Errorf("Invalid issue config")
		}
	}

	if cfg.ToOpen != "" {
		err = exec.Command("open", cfg.ToOpen).Run()
	}

	if cfg.ToUpdate != nil {
		toUpdate := *cfg.ToUpdate
		session := OpenSession(config.RedmineURL, config.APIKey)

		if err = session.UpdateIssue(toUpdate.ID, toUpdate.Issue); err != nil {
			return
		}

		var issue Issue
		if issue, err = session.GetIssue(toUpdate.ID); err != nil {
			return
		}

		for i := range cache.Issues {
			if cache.Issues[i].ID == issue.ID {
				cache.Issues[i] = issue
				if err := alfred.SaveJSON(cacheFile, &cache); err != nil {
					log.Printf("Error saving cache: %v\n", err)
				}
				break
			}
		}

		out = fmt.Sprintf("Updated issue %d", toUpdate.ID)
	}

	return
}

// support -------------------------------------------------------------------

const issuesKeyword = "issues"

type issueCfg struct {
	IssueID   *int
	ProjectID *int
	ToUpdate  *updateIssueMessage
	ToOpen    string
}

type updateIssueMessage struct {
	Action string
	ID     int
	Issue  UpdateIssue
}

func createIssueItems(arg string, pid int, issues []Issue) (items []alfred.Item) {
	var filtered []Issue

	for i := range issues {
		if pid != -1 && pid != issues[i].Project.ID {
			// If a project ID was given, only use issues for that project
			continue
		}

		if alfred.FuzzyMatches(issues[i].Subject, arg) || alfred.FuzzyMatches(strconv.Itoa(issues[i].ID), arg) {
			filtered = append(filtered, issues[i])
		}
	}

	sort.Sort(byDueDate(filtered))
	sort.Stable(sort.Reverse(byPriority(filtered)))
	sort.Stable(byAssignment(filtered))

	for i := range filtered {
		items = append(items, filtered[i].toItem())
	}

	return
}

func getMyIssuesURL() string {
	return config.RedmineURL + "/issues?utf8=✓&set_filter=1&" +
		"f[]=assigned_to_id&op[assigned_to_id]==&v[assigned_to_id][]=me&" +
		"f[]=status_id&op[status_id]=o&f[]=&c[]=project&c[]=status&c[]=priority&c[]=subject&" +
		"c[]=updated_on&c[]=due_date&c[]=estimated_hours&c[]=spent_hours&c[]=done_ratio&group_by="
}

func (i *Issue) toItem() (item alfred.Item) {
	subTitle := fmt.Sprintf("%d [%s]", i.ID, i.Project.Name)
	if i.DueDate != "" {
		dueDate, _ := time.Parse("2006-01-02", i.DueDate)
		subTitle += " Due " + toHumanDateString(dueDate) + ","
	}
	subTitle += " " + i.Priority.Name

	item.Title = i.Subject
	item.Subtitle = subTitle
	item.Autocomplete = strconv.Itoa(i.ID)

	url := fmt.Sprintf("%s/issues/%v", config.RedmineURL, i.ID)
	item.Arg = &alfred.ItemArg{
		Keyword: issuesKeyword,
		Mode:    alfred.ModeDo,
		Data:    alfred.Stringify(&issueCfg{ToOpen: url}),
	}

	if i.AssignedTo.ID == cache.User.ID {
		item.Icon = "icon_me.png"
	}

	return
}

func getIssueByID(id int) (issue Issue, err error) {
	for _, i := range cache.Issues {
		if i.ID == id {
			issue = i
			return
		}
	}

	return issue, fmt.Errorf("Invalid ID %d", id)
}
