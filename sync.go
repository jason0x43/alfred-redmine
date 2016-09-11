package main

import "github.com/jason0x43/go-alfred"

// SyncCommand is a command
type SyncCommand struct{}

// About returns information about a command
func (c SyncCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "sync",
		Description: "Sync with your Redmine server",
		IsEnabled:   config.APIKey != "",
	}
}

// Items returns a list of filter items
func (c SyncCommand) Items(arg, data string) (items []alfred.Item, err error) {
	if err = refresh(); err != nil {
		return
	}
	items = append(items, alfred.Item{Title: "Synchronized!"})
	return
}
