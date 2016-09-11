package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
)

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

	session := OpenSession(config.RedmineURL, config.APIKey)

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
			case User:
				cache.User = value
				log.Println("Got users")
			case []Issue:
				cache.Issues = value
				log.Println("Got issues")
			case []IssueStatus:
				cache.IssueStatuses = value
				log.Println("Got issue statuses")
			case []Project:
				cache.Projects = value
				log.Println("Got projects")
			case []TimeEntry:
				cache.TimeEntries = value
				log.Println("Got time entries")
			}
		case err := <-errorChan:
			return err
		}
	}

	cache.Time = time.Now()
	err := alfred.SaveJSON(cacheFile, &cache)
	if err != nil {
		log.Printf("Error writing cache: %s", err)
	}

	return nil
}

func indexOfByName(list listInterface, name string) int {
	name = strings.ToLower(name)
	for i := 0; i < list.Len(); i++ {
		if strings.ToLower(list.Name(i)) == name {
			return i
		}
	}
	return -1
}

func indexOfByID(list listInterface, id int) int {
	for i := 0; i < list.Len(); i++ {
		if list.ID(i) == id {
			return i
		}
	}
	return -1
}

type listInterface interface {
	Len() int
	Name(index int) string
	ID(index int) int
}

type tsProjectList []timesheetProject

func (l tsProjectList) Len() int {
	return len(l)
}
func (l tsProjectList) Name(index int) string {
	return l[index].name
}
func (l tsProjectList) ID(index int) int {
	return l[index].id
}

type projectList []Project

func (l projectList) Len() int {
	return len(l)
}
func (l projectList) Name(index int) string {
	return l[index].Name
}
func (l projectList) ID(index int) int {
	return l[index].ID
}

type issueList []Issue

func (l issueList) Len() int {
	return len(l)
}
func (l issueList) Name(index int) string {
	return l[index].Description
}
func (l issueList) ID(index int) int {
	return l[index].ID
}

func getClosedStatusIDs() map[int]bool {
	closed := map[int]bool{}
	for _, status := range cache.IssueStatuses {
		if status.IsClosed {
			closed[status.ID] = true
		}
	}
	return closed
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
	}
	return toIsoDateString(date)
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

func dueDateIsBefore(i, j string) bool {
	if i == "" {
		return false
	}
	if j == "" {
		return true
	}
	return i < j
}

type byDueDate []Issue

func (b byDueDate) Len() int {
	return len(b)
}

func (b byDueDate) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byDueDate) Less(i, j int) bool {
	return dueDateIsBefore(b[i].DueDate, b[j].DueDate)
}

type byAssignment []Issue

func (b byAssignment) Len() int {
	return len(b)
}

func (b byAssignment) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byAssignment) Less(i, j int) bool {
	if b[i].AssignedTo.ID == cache.User.ID && b[j].AssignedTo.ID != cache.User.ID {
		return true
	}
	return false
}

type byPriority []Issue

func (b byPriority) Len() int {
	return len(b)
}

func (b byPriority) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byPriority) Less(i, j int) bool {
	return b[i].Priority.ID < b[j].Priority.ID
}
