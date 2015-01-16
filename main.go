package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-redmine"
)

var cacheFile string
var configFile string
var config Config
var cache Cache
var workflow *alfred.Workflow

type Config struct {
	ApiKey     string
	RedmineUrl string
}

func main() {
	var err error

	workflow, err = alfred.OpenWorkflow(".", true)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	log.Println("Using config file", configFile)
	log.Println("Using cache file", cacheFile)

	err = alfred.LoadJson(configFile, &config)
	if err != nil {
		log.Println("Error loading config:", err)
	}

	err = alfred.LoadJson(cacheFile, &cache)
	if err != nil {
		log.Println("Error loading cache:", err)
	}

	commands := []alfred.Command{
		IssuesCommand{},
		ProjectsCommand{},
		TimesheetCommand{},
		SyncCommand{},
		OpenAction{},
		LoginCommand{},
		LogoutCommand{},
		ServerCommand{},
	}

	workflow.Run(commands)
}

// login -------------------------------------------------

type LoginCommand struct{}

func (c LoginCommand) Keyword() string {
	return "login"
}

func (c LoginCommand) IsEnabled() bool {
	return config.ApiKey == "" && config.RedmineUrl != ""
}

func (c LoginCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Arg:          "login",
		SubtitleAll:  "Login to your Redmine server",
	}
}

func (c LoginCommand) Items(prefix, query string) ([]alfred.Item, error) {
	return []alfred.Item{c.MenuItem()}, nil
}

func (c LoginCommand) Do(query string) (string, error) {
	btn, username, err := workflow.GetInput("Username", "", false)
	if err != nil {
		return "", err
	}

	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("username: %s", username)

	btn, password, err := workflow.GetInput("Password", "", true)
	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	log.Printf("password: *****")

	session, err := redmine.NewSession(config.RedmineUrl, username, password)
	if err != nil {
		workflow.ShowMessage(fmt.Sprintf("Login failed: %s", err))
		return "", nil
	}

	config.ApiKey = session.ApiKey()
	err = alfred.SaveJson(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("Login successful!")
	return "", nil
}

// logout ------------------------------------------------

type LogoutCommand struct{}

func (c LogoutCommand) Keyword() string {
	return "logout"
}

func (c LogoutCommand) IsEnabled() bool {
	return config.ApiKey != ""
}

func (c LogoutCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword(),
		Arg:          "logout",
		SubtitleAll:  "Logout of your Redmine server",
	}
}

func (c LogoutCommand) Items(prefix, query string) ([]alfred.Item, error) {
	item := c.MenuItem()
	item.Arg = "logout"
	return []alfred.Item{item}, nil
}

func (c LogoutCommand) Do(query string) (string, error) {
	config.ApiKey = ""
	err := alfred.SaveJson(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("Logout successful!")
	return "", nil
}

// config ------------------------------------------------

type ServerCommand struct{}

func (c ServerCommand) Keyword() string {
	return "server"
}

func (c ServerCommand) IsEnabled() bool {
	return true
}

func (c ServerCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		SubtitleAll:  "Address of your Redmine server",
	}
}

func (c ServerCommand) Items(prefix, query string) (items []alfred.Item, err error) {
	if query == "" {
		if config.RedmineUrl != "" {
			items = append(items, alfred.Item{
				Title:       "Current server",
				SubtitleAll: config.RedmineUrl,
				Valid:       alfred.Invalid,
			})
		} else {
			items = append(items, alfred.Item{
				Title:       "Use server at...",
				SubtitleAll: "Enter a URL",
				Valid:       alfred.Invalid,
			})
		}
	} else {
		item := alfred.Item{
			Title:       "Use server at...",
			SubtitleAll: query,
		}

		url, err := url.Parse(query)
		if err == nil {
			item.Arg = "server " + url.String()
			item.Valid = ""
		}

		items = append(items, item)
	}

	return items, nil
}

func (c ServerCommand) Do(query string) (string, error) {
	config.RedmineUrl = query
	err := alfred.SaveJson(configFile, &config)
	if err != nil {
		return "", err
	}

	workflow.ShowMessage("Using server at " + query)
	return "", nil
}

// timesheet -----------------------------------------------------------------

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

// issues --------------------------------------------------------------------

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

// projects ------------------------------------------------------------------

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

// projects ------------------------------------------------------------------

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

// open ----------------------------------------------------------------------

type OpenAction struct{}

func (a OpenAction) Keyword() string {
	return "open"
}

func (c OpenAction) IsEnabled() bool {
	return true
}

func (a OpenAction) Do(query string) (string, error) {
	log.Printf("Opening URL " + query)
	err := exec.Command("open", query).Run()
	return "", err
}

// support functions ---------------------------------------------------------

func checkRefresh() error {
	if time.Now().Sub(cache.Time).Minutes() <= 10.0 {
		return nil
	}

	log.Println("Refreshing cache...")
	err := refresh()
	if err != nil {
		log.Println("Error refreshing cache:", err)
	}
	return err
}

func refresh() error {
	dataChan := make(chan interface{})
	errorChan := make(chan error)

	session := redmine.OpenSession(config.RedmineUrl, config.ApiKey)

	log.Println("Getting user...")
	go func() {
		user, err := session.GetUser()
		if err != nil {
			errorChan <- err
		} else {
			dataChan <- user
		}
	}()

	log.Println("Getting issues...")
	go func() {
		issues, err := session.GetIssues()
		if err != nil {
			errorChan <- err
		} else {
			dataChan <- issues
		}
	}()

	log.Println("Getting statuses...")
	go func() {
		statuses, err := session.GetIssueStatuses()
		if err != nil {
			errorChan <- err
		} else {
			dataChan <- statuses
		}
	}()

	log.Println("Getting projects...")
	go func() {
		projects, err := session.GetProjects()
		if err != nil {
			errorChan <- err
		} else {
			dataChan <- projects
		}
	}()

	log.Println("Getting time entries...")
	go func() {
		timeEntries, err := session.GetTimeEntries(7)
		if err != nil {
			errorChan <- err
		} else {
			dataChan <- timeEntries
		}
	}()

	// wait for 5 items to come in
	for i := 0; i < 5; i++ {
		select {
		case data := <-dataChan:
			switch value := data.(type) {
			case *redmine.User:
				cache.User = value
				log.Println("Got users")
			case []redmine.Issue:
				cache.Issues = value
				log.Println("Got issues")
			case []redmine.IssueStatus:
				cache.IssueStatuses = value
				log.Println("Got issue statuses")
			case []redmine.Project:
				cache.Projects = value
				log.Println("Got projects")
			case []redmine.TimeEntry:
				cache.TimeEntries = value
				log.Println("Got time entries")
			}
		case err := <-errorChan:
			return err
		}
	}

	cache.Time = time.Now()
	err := alfred.SaveJson(cacheFile, &cache)
	if err != nil {
		log.Printf("Error writing cache: %s", err)
	}

	return nil
}

func indexOfByName(list ListInterface, name string) int {
	name = strings.ToLower(name)
	for i := 0; i < list.Len(); i++ {
		if strings.ToLower(list.Name(i)) == name {
			return i
		}
	}
	return -1
}

func indexOfById(list ListInterface, id int) int {
	for i := 0; i < list.Len(); i++ {
		if list.Id(i) == id {
			return i
		}
	}
	return -1
}

type ListInterface interface {
	Len() int
	Name(index int) string
	Id(index int) int
}

type ProjectList []*timesheetProject

func (l ProjectList) Len() int {
	return len(l)
}
func (l ProjectList) Name(index int) string {
	return l[index].name
}
func (l ProjectList) Id(index int) int {
	return l[index].id
}

type RmProjectList []redmine.Project

func (l RmProjectList) Len() int {
	return len(l)
}
func (l RmProjectList) Name(index int) string {
	return l[index].Name
}
func (l RmProjectList) Id(index int) int {
	return l[index].Id
}

type IssueList []redmine.Issue

func (l IssueList) Len() int {
	return len(l)
}
func (l IssueList) Name(index int) string {
	return l[index].Description
}
func (l IssueList) Id(index int) int {
	return l[index].Id
}

func getClosedStatusIds() map[int]bool {
	closed := map[int]bool{}
	for _, status := range cache.IssueStatuses {
		if status.IsClosed {
			closed[status.Id] = true
		}
	}
	return closed
}

func getTimesheetUrl(since, until string) string {
	addrFormat := config.RedmineUrl + "/timesheet/report" +
		"?timesheet[date_from]=%v" +
		"&timesheet[date_to]=%v" +
		"&timesheet[sort]=project" +
		"&timesheet[users][]=%v"

	return fmt.Sprintf(addrFormat, since, until, cache.User.Id)
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

func getMissingIssues() error {
	issueChan := make(chan *redmine.Issue)
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
			cache.Issues = append(cache.Issues, *issue)
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

func toIsoDateString(date time.Time) string {
	return fmt.Sprintf("%d-%02d-%02d", date.Year(), date.Month(), date.Day())
}

func toHumanDateString(date time.Time) string {
	today := time.Now()

	if isSameDate(date, today) {
		return "today"
	} else if isSameDate(date, today.AddDate(0, 0, -1)) {
		return "yesterday"
	} else if isSameDate(date, today.AddDate(0, 0, +1)) {
		return "tomorow"
	} else if isDateAfter(date, today.AddDate(0, 0, -7)) && isLastWeek(date, today) {
		return "last " + date.Weekday().String()
	} else if isDateBefore(date, today.AddDate(0, 0, 7)) {
		return date.Weekday().String()
	} else if isNextWeek(date, today) {
		return "next " + date.Weekday().String()
	} else if isDateBefore(date, today.AddDate(1, 0, 0)) {
		return date.Format("Jan 2")
	} else {
		return toIsoDateString(date)
	}
}

// is date1's date before date2's date
func isDateBefore(date1 time.Time, date2 time.Time) bool {
	return date1.Year() < date2.Year() || (date1.Year() == date2.Year() && date1.YearDay() < date2.YearDay())
}

// is date1's date after date2's date
func isDateAfter(date1 time.Time, date2 time.Time) bool {
	return date1.Year() > date2.Year() || (date1.Year() == date2.Year() && date1.YearDay() > date2.YearDay())
}

// do date1 and date2 refer to the same date
func isSameDate(date1 time.Time, date2 time.Time) bool {
	return date1.Year() == date2.Year() && date1.YearDay() == date2.YearDay()
}

func isLastWeek(date1 time.Time, date2 time.Time) bool {
	y1, w1 := date1.ISOWeek()
	y2, w2 := date2.ISOWeek()
	return y1 == y2 && w1 == w2-1
}

func isSameWeek(date1 time.Time, date2 time.Time) bool {
	y1, w1 := date1.ISOWeek()
	y2, w2 := date2.ISOWeek()
	return y1 == y2 && w1 == w2
}

func isNextWeek(date1 time.Time, date2 time.Time) bool {
	y1, w1 := date1.ISOWeek()
	y2, w2 := date2.ISOWeek()
	return y1 == y2 && w1 == w2+1
}

type Cache struct {
	Time          time.Time
	User          *redmine.User
	Issues        []redmine.Issue
	IssueStatuses []redmine.IssueStatus
	Projects      []redmine.Project
	TimeEntries   []redmine.TimeEntry
}

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

func dueDateIsBefore(i, j *redmine.Issue) bool {
	if i.DueDate == "" {
		return false
	}
	if j.DueDate == "" {
		return true
	}
	return i.DueDate < j.DueDate
}

type byDueDate []redmine.Issue

func (b byDueDate) Len() int {
	return len(b)
}

func (b byDueDate) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byDueDate) Less(i, j int) bool {
	return dueDateIsBefore(&b[i], &b[j])
}

type byAssignment []redmine.Issue

func (b byAssignment) Len() int {
	return len(b)
}

func (b byAssignment) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byAssignment) Less(i, j int) bool {
	if b[i].AssignedTo.Id == cache.User.Id && b[j].AssignedTo.Id != cache.User.Id {
		return true
	}
	return false
}

type byPriority []redmine.Issue

func (b byPriority) Len() int {
	return len(b)
}

func (b byPriority) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byPriority) Less(i, j int) bool {
	return b[i].Priority.Id < b[j].Priority.Id
}
