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
			log.Println("bad status")
			b.Send(m.Sender, "bad status")
			return
		}

		var recur bool
		switch recurring {
		case "yes":
			recur = true
		case "no":
			recur = false
		default:
			log.Println("bad recurring")
			b.Send(m.Sender, "bad recurring")
			return
		}

		t, err := parseTime(when)
		if err != nil {
			log.Println("bad time")
			b.Send(m.Sender, "bad time")
			return
		}

		task := schedule.Task{
			When:      t,
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
		if c.Data == "" {
			b.Respond(c)
			return
		}

		var msg string
		if s.Remove(c.Data) {
			msg = "task deleted"
		} else {
			msg = "failed to delete task, is it already gone?"
		}
		// this or b.Delete(c.Message)
		origMsg := c.Message
		b.Edit(origMsg, "Deleted task\n"+origMsg.Text, &tb.ReplyMarkup{})

		b.Respond(c, &tb.CallbackResponse{
			Text: msg,
		})
	})

	b.Start()
}
