package main

import (
	"fmt"
	"log"
	"strings"
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

func getAddKeyboard(repr taskRepr) *tb.ReplyMarkup {
	marsh, _ := repr.MarshalText()
	data := string(marsh)
	keyboard := [][]tb.InlineButton{
		{
			*with(addApiStatus, data),
			*with(addApiOn, data),
			*with(addApiOff, data),
		},
		{
			*with(addRecurring, data),
		},
		{
			*with(addCancel, data),
			*with(addDone, data),
		},
	}
	for i, b := range keyboard[0] {
		//color := "\U0001F534  "
		color := "\u26aa  "
		if b.Unique == repr.what {
			color = "\U0001F7E2  "
		}
		keyboard[0][i].Text = color + keyboard[0][i].Text
	}
	if repr.recurring {
		keyboard[1][0].Text = "\U0001F501  " + keyboard[1][0].Text
	}
	return &tb.ReplyMarkup{InlineKeyboard: keyboard}
}

func parseTime(s string) (time.Time, error) {
	parts := strings.Split(s, " ")
	t, err := time.ParseInLocation("15:04", parts[len(parts)-1], time.Local)
	if err != nil {
		return time.Time{}, err
	}
	if len(parts) != 2 {
		return schedule.NextTime(t.Hour(), t.Minute()), nil
	}
	date, err := time.Parse("2/1", parts[0])
	if err != nil {
		return time.Time{}, err
	}
	return schedule.NextDate(date.Month(), date.Day(), t.Hour(), t.Minute()), nil
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
			a.Status() // set LastKnownState to known good value
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

	setHandlers(b, a, s)
	b.Start()
}
