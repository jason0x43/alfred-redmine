package main

import (
	"fmt"
	"log"
	"os"
	"path"
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

type Cache struct {
	Time          time.Time
	User          redmine.User
	Issues        []redmine.Issue
	IssueStatuses []redmine.IssueStatus
	Projects      []redmine.Project
	TimeEntries   []redmine.TimeEntry
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
