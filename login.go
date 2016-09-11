package main

import (
	"fmt"
	"log"

	"github.com/jason0x43/go-alfred"
)

// LoginCommand is a command
type LoginCommand struct{}

// About returns information about a command
func (c LoginCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "login",
		Description: "Login to your Redmine server",
		IsEnabled:   config.APIKey == "",
		Arg: &alfred.ItemArg{
			Keyword: "login",
			Mode:    alfred.ModeDo,
		},
	}
}

// Do runs the command
func (c LoginCommand) Do(data string) (out string, err error) {
	var btn string
	var urlStr string

	if btn, urlStr, err = workflow.GetInput("Redmine server URL", "", false); err != nil {
		return
	}
	if btn != "Ok" {
		dlog.Println("User didn't click OK")
		return
	}
	dlog.Printf("URL: %s", urlStr)

	var username string
	if btn, username, err = workflow.GetInput("Username", "", false); err != nil {
		return "", err
	}
	if btn != "Ok" {
		dlog.Println("User didn't click OK")
		return
	}
	dlog.Printf("username: %s", username)

	var password string
	btn, password, err = workflow.GetInput("Password", "", true)
	if btn != "Ok" {
		log.Println("User didn't click OK")
		return "", nil
	}
	dlog.Printf("password: *****")

	var session Session
	if session, err = NewSession(urlStr, username, password); err != nil {
		workflow.ShowMessage(fmt.Sprintf("Login failed: %s", err))
		return
	}

	config.RedmineURL = urlStr
	config.APIKey = session.APIKey()
	if err = alfred.SaveJSON(configFile, &config); err != nil {
		return
	}

	workflow.ShowMessage("Login successful!")
	return
}
