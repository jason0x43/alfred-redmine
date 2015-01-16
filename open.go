package main

import (
	"log"
	"os/exec"
)

type OpenAction struct{}

func (a OpenAction) Keyword() string {
	return "open"
}

func (c OpenAction) IsEnabled() bool {
	return true
}

func (a OpenAction) Do(query string) (string, error) {
	log.Printf("Opening URL " + query)
	err := exec.Command("open", query).Run()
	return "", err
}
