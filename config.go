package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/profawk/espurnaBot/bot"
	"os"
)

var config struct {
	BotToken string
	ChatIds  []int64
	Watchdog bool
	Espurna  struct {
		Relay    int `json:",omitempty"`
		Hostname string
		ApiKey   string
	}
	Triggers bot.Triggers
}

var filePath = flag.String("c", "config.json", "config file path")

func init() {
	flag.Parse()
	if _, err := os.Stat(*filePath); os.IsNotExist(err) {
		fmt.Println("config file not found")
		flag.Usage()
		os.Exit(1)
	}
	f, err := os.Open(*filePath)
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
