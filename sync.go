package main

import "github.com/jason0x43/go-alfred"

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
