package main

import (
	"fmt"
	"github.com/profawk/espurnaBot/bot"
	"log"
	"time"

	"github.com/profawk/espurnaBot/api"
	"github.com/profawk/espurnaBot/schedule"

	tb "gopkg.in/tucnak/telebot.v2"
)

func validateChatIds(upd *tb.Update) bool {
	if upd.Message == nil {
		return true
	}

	chatID := upd.Message.Chat.ID
	for _, allowedID := range config.ChatIds {
		if allowedID == chatID {
			return true
		}
	}

	log.Printf("chat ID %d is not in allowed chat ids", chatID)
	return false
}

func main() {
	a := api.NewAPI(config.Espurna.ApiKey, config.Espurna.Hostname, config.Espurna.Relay)
	s := schedule.NewSchedule()

	poller := &tb.LongPoller{Timeout: 10 * time.Second}
	privateBotPoller := tb.NewMiddlewarePoller(poller, validateChatIds)
	b, err := tb.NewBot(tb.Settings{
		Token:  config.BotToken,
		Poller: privateBotPoller,
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	if config.Watchdog {
		t := time.NewTicker(1 * time.Minute)
		go func() {
			for range t.C {
				// set LastKnownState to known good value
				_, err := a.Status()
				// loop until no error
				if err == nil {
					break
				}
				log.Print("error while starting watchdog", err, "retrying")
			}
			for range t.C {
				s := a.LastKnownState
				newS, err := a.Status()
				if err != nil {
					log.Print("error in watchdog", err)
				}
				if newS == s {
					continue
				}
				for _, chatId := range config.ChatIds {
					b.Send(
						tb.ChatID(chatId),
						fmt.Sprintf("relay state has changed. it is now %s", api.StateNames[newS]),
					)
				}

			}
		}()
	}

	bot.SetHandlers(b, a, s)
	b.Start()
}
