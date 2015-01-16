package main

import (
	"net/url"

	"github.com/jason0x43/go-alfred"
)

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
