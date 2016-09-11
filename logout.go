package main

import "github.com/jason0x43/go-alfred"

// LogoutCommand is a command
type LogoutCommand struct{}

// About returns information about a command
func (c LogoutCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "logout",
		Description: "Logout of your Redmine server",
		IsEnabled:   config.APIKey != "",
		Arg: &alfred.ItemArg{
			Keyword: "logout",
			Mode:    alfred.ModeDo,
		},
	}
}

// Do runs the command
func (c LogoutCommand) Do(data string) (out string, err error) {
	config.APIKey = ""
	if err = alfred.SaveJSON(configFile, &config); err != nil {
		return
	}

	workflow.ShowMessage("Logout successful!")
	return
}
