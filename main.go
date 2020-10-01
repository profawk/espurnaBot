package main

import (
	"fmt"
	"github.com/profawk/espurnaBot/api"
	"log"
	"time"

	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	// Universal markup builders.
	menu        = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnStatus   = menu.Text("ℹ Status")
	btnOn       = menu.Text("⚡ Power On")
	btnOff      = menu.Text("🔌 Power Off")
	btnSchedule = menu.Text("⚙ Schedule")
)

func init() {
	menu.Reply(
		menu.Row(btnStatus),
		menu.Row(btnOn, btnOff),
		menu.Row(btnSchedule),
	)
}

func validateChatIds(upd *tb.Update) bool {
	if upd.Message == nil {
		return true
	}

	for _, cid := range config.ChatIds {
		if cid == upd.Message.Chat.ID {
			return true
		}
	}

	return false
}

func apiMiddleware(b *tb.Bot, apiCall func() api.State) func(m *tb.Message) {
	return func(m *tb.Message) {
		var msg string
		if apiCall() {
			msg = "on"
		} else {
			msg = "off"
		}
		_, err := b.Send(m.Sender, fmt.Sprintf("The relay is %s", msg), menu)
		if err != nil {
			log.Print("status: ", err)
		}
	}
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

	b.Handle("/start", func(m *tb.Message) {
		if !m.Private() {
			return
		}

		b.Send(m.Sender, "Here is the menu", menu)
	})

	b.Handle(&btnStatus, apiMiddleware(b, a.Status))
	b.Handle(&btnOn, apiMiddleware(b, a.TurnOn))
	b.Handle(&btnOff, apiMiddleware(b, a.TurnOff))

	b.Start()
}
