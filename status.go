package main

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/jason0x43/go-alfred"
)

// StatusCommand is a command
type StatusCommand struct{}

// About returns information about this command
func (c StatusCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "status",
		Description: "Show current status",
		IsEnabled:   config.APIKey != "",
	}
}

// Items returns a list of filter items
func (c StatusCommand) Items(arg, data string) (items []alfred.Item, err error) {
	dlog.Printf("status items with arg=%s, data=%s", arg, data)

	if latest, available := workflow.UpdateAvailable(); available {
		items = append(items, alfred.Item{
			Title:    fmt.Sprintf("Update available: %v", latest.Version),
			Subtitle: fmt.Sprintf("You have %s", workflow.Version()),
			Arg: &alfred.ItemArg{
				Keyword: "status",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(statusCfg{ToOpen: latest.URL}),
			},
		})
	} else {
		items = append(items, alfred.Item{
			Title: fmt.Sprintf("You have the latest version of %s", workflow.Name()),
		})
	}

	if err = checkRefresh(); err != nil {
		items = append(items, alfred.Item{
			Title:    "Error syncing with redmine",
			Subtitle: fmt.Sprintf("%v", err),
		})
	} else {
		closed := getClosedStatusIDs()
		for _, issue := range cache.Issues {
			if _, isClosed := closed[issue.Status.ID]; !isClosed {
				if issue.AssignedTo.ID == cache.User.ID {
					items = append(items, issue.toItem())
				}
			}
		}
	}

	return
}

// Do runs the command
func (c StatusCommand) Do(data string) (out string, err error) {
	var cfg statusCfg

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshaling tag data: %v", err)
		}
	}

	if cfg.ToOpen != "" {
		dlog.Printf("opening %s", cfg.ToOpen)
		err = exec.Command("open", cfg.ToOpen).Run()
	}

	return
}

type statusCfg struct {
	ToOpen string
}
