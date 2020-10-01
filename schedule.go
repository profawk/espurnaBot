package main

import (
	"github.com/profawk/espurnaBot/api"
	tb "gopkg.in/tucnak/telebot.v2"
)

func apiTaskAdapter(b *tb.Bot, dest tb.Recipient, apiCall func() api.State) func() {
	return func() {
		sendApiMessage(b, dest, "Task executed\nThe relay is %s", apiCall)
	}
}
