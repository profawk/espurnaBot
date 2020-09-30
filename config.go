package main

import (
	"encoding/json"
	"os"
)

var config struct {
	BotToken string
	ChatIds []int64
	Espurna struct {
		Relay    int `json:",omitempty"`
		Hostname string
		ApiKey   string
	}
}

func init() {
	f, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	if err = json.NewDecoder(f).Decode(&config); err != nil {
		panic(err)
	}
	if config.BotToken == "" || config.Espurna.ApiKey == "" || config.Espurna.Hostname == "" || len(config.ChatIds) == 0 {
		panic("json invalid")
	}
	// if not relay specified 0 should be chosen. done with go zero value
}
