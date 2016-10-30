package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/jason0x43/go-alfred"
)

var cacheFile string
var configFile string
var workflow alfred.Workflow
var config struct {
	APIKey          string `desc:"Server API key"`
	RedmineURL      string `desc:"Server URL"`
	AllowSelfSigned bool   `desc:"If true, accept self-signed SSL certificates"`
}
var cache struct {
	Time          time.Time
	User          User
	Issues        []Issue
	IssueStatuses []IssueStatus
	Projects      []Project
	TimeEntries   []TimeEntry
}

var dlog = log.New(os.Stderr, "[redmine] ", log.LstdFlags)

func main() {
	var err error
	if workflow, err = alfred.OpenWorkflow(".", true); err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	log.Println("Using config file", configFile)
	log.Println("Using cache file", cacheFile)

	if err := alfred.LoadJSON(configFile, &config); err != nil {
		log.Println("Error loading config:", err)
	}

	if err := alfred.LoadJSON(cacheFile, &cache); err != nil {
		log.Println("Error loading cache:", err)
	}

	AllowSelfSignedCert(config.AllowSelfSigned)

	workflow.Run([]alfred.Command{
		IssuesCommand{},
		ProjectsCommand{},
		TimesheetCommand{},
		SyncCommand{},
		OptionsCommand{},
		LoginCommand{},
		LogoutCommand{},
	})
}
