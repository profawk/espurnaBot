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

var (
	menu        = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnStatus   = menu.Text("ℹ Status")
	btnOn       = menu.Text("⚡ Power On")
	btnOff      = menu.Text("🔌 Power Off")
	btnSchedule = menu.Text("⚙ Schedule")

	delKeyboard = &tb.ReplyMarkup{}
	delInline   = delKeyboard.Data("🗑️ Cancel", "delete").Inline()
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

func parseTime(s string) time.Time {
	t, err := time.ParseInLocation("02/01 03:04", s, time.Local)
	if err == nil {
		return time.Date(time.Now().Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
	}
	t, err = time.ParseInLocation("03:04", s, time.Local)
	if err != nil {
		log.Printf("can't parse time")
		return time.Time{}
	}
	return schedule.Next(t.Hour(), t.Minute())
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

	b.Handle("/help", func(m *tb.Message) {
		b.Send(m.Sender, "/start to start\n/schedule to add a schedule - /schedule 20/12 10:25,off,yes", menu)
	})

	b.Handle("/schedule", func(m *tb.Message) {
		parts := strings.Split(m.Payload, ",")
		when, what, recurring := parts[0], parts[1], parts[2]

		var f func() api.State
		var desc string
		switch what {
		case "on":
			f = a.TurnOn
			desc = "turn on"
		case "off":
			f = a.TurnOff
			desc = "turn off"
		case "status":
			f = a.Status
			desc = "get status"
		default:
			log.Printf("bad status")
		}

		var recur bool
		switch recurring {
		case "yes":
			recur = true
		case "no":
			recur = false
		default:
			log.Printf("bad recurring")
		}

		task := schedule.Task{
			When:      parseTime(when),
			What:      apiTaskAdapter(b, m.Sender, f),
			Desc:      desc,
			Recurring: recur,
		}
		s.Add(task)

		b.Send(m.Sender, "Task has been added\n"+task.String(), menu)
	})

	b.Handle(&btnStatus, apiMiddleware(b, a.Status))
	b.Handle(&btnOn, apiMiddleware(b, a.TurnOn))
	b.Handle(&btnOff, apiMiddleware(b, a.TurnOff))

	b.Handle(&btnSchedule, func(m *tb.Message) {
		s.L.Lock()
		defer s.L.Unlock()
		for id, t := range s.Tasks {
			btn := delInline.With(id)
			btn.URL = "" // until https://github.com/tucnak/telebot/pull/324 gets approved
			delKeyboard.InlineKeyboard = [][]tb.InlineButton{{*btn}}
			b.Send(m.Sender, t.String(), delKeyboard)
		}
		if len(s.Tasks) == 0 {
			b.Send(m.Sender, "Schedule is empty")
		}
	})

	b.Handle(delInline, func(c *tb.Callback) {
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
