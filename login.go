package main

import (
	"fmt"
	"log"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-redmine"
)

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
