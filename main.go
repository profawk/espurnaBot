package main

import (
	"fmt"
	"log"
	"time"

	"github.com/profawk/espurnaBot/api"
	"github.com/profawk/espurnaBot/schedule"

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

func sendApiMessage(b *tb.Bot, dest tb.Recipient, msg string, apiCall func() api.State) {
	var stateStr string
	if apiCall() {
		stateStr = "on"
	} else {
		stateStr = "off"
	}
	_, err := b.Send(dest, fmt.Sprintf(msg, stateStr), menu)
	if err != nil {
		log.Print("status: ", err)
	}
}
func apiMiddleware(b *tb.Bot, apiCall func() api.State) func(m *tb.Message) {
	return func(m *tb.Message) {
		sendApiMessage(b, m.Sender, "The relay is %s", apiCall)
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
		b.Send(m.Sender, "Here is the menu", menu)
	})

	b.Handle(&btnStatus, apiMiddleware(b, a.Status))
	b.Handle(&btnOn, apiMiddleware(b, a.TurnOn))
	b.Handle(&btnOff, apiMiddleware(b, a.TurnOff))

	b.Handle(&btnSchedule, func(m *tb.Message) {
		delKeyboard := &tb.ReplyMarkup{}
		s.L.Lock()
		defer s.L.Unlock()
		for id, t := range s.Tasks {
			delInline := delKeyboard.Data("delete", "delete", id)
			delKeyboard.Inline(
				delKeyboard.Row(delInline),
			)
			b.Send(m.Sender, t.String(), delKeyboard)
		}
		if len(s.Tasks) == 0 {
			b.Send(m.Sender, "Schedule is empty")
		}
	})

	// hacky handler
	b.Handle("\fdelete", func(c *tb.Callback) {
		var msg string
		if c.Data == "" {
			msg = ""
		}

		if s.Remove(c.Data) {
			msg = "task deleted"
		} else {
			msg = "failed to delete task, is it already gone?"
		}

		b.Respond(c, &tb.CallbackResponse{
			Text: msg,
		})
	})
	b.Start()
}
