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
	btnAddTask  = menu.Text("➕ Add Task")

	delKeyboard = &tb.ReplyMarkup{}
	delInline   = delKeyboard.Data("🗑️ Cancel", "delete").Inline()

	addKeyboard  = &tb.ReplyMarkup{}
	addApiStatus = addKeyboard.Data("Get status", "status").Inline()
	addApiOn     = addKeyboard.Data("Turn on", "on").Inline()
	addApiOff    = addKeyboard.Data("Turn off", "off").Inline()
	addRecurring = addKeyboard.Data("Recurring", "recurring").Inline()
	addDone      = addKeyboard.Data("Done  ✔️", "done").Inline()
	addCancel    = addKeyboard.Data("Cancel  ❌", "cancel").Inline()
)

func init() {
	menu.Reply(
		menu.Row(btnStatus),
		menu.Row(btnOn, btnOff),
		menu.Row(btnSchedule, btnAddTask),
	)
}

// until https://github.com/tucnak/telebot/pull/324 gets approved
func with(b *tb.InlineButton, data string) *tb.InlineButton {
	nb := b.With(data)
	nb.URL = b.URL
	return nb
}

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

func sendApiMessage(b *tb.Bot, dest tb.Recipient, msg string, apiCall func() api.State) {
	stateStr := api.StateNames[apiCall()]
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

	addApiButtons := map[string]struct {
		what func() api.State
		desc string
	}{
		addApiStatus.Unique: {a.Status, "Get status"},
		addApiOn.Unique:     {a.TurnOn, "Turn on relay"},
		addApiOff.Unique:    {a.TurnOff, "Turn off relay"},
	}
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
				newS := a.Status()
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

	b.Handle("/start", func(m *tb.Message) {
		b.Send(m.Sender, "Here is the menu", menu)
	})

	b.Handle("/help", func(m *tb.Message) {
		b.Send(m.Sender, "/start to start\n/schedule to add a schedule - /schedule 20/12 10:25", menu)
	})

	b.Handle("/schedule_full", func(m *tb.Message) {
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

	newTaskHandler := func(m *tb.Message) {
		t, err := parseTime(m.Payload)
		if err != nil {
			b.Send(m.Sender, "Could not parse time. is it in [dd/mm] hh:mm")
			log.Print(err)
			return
		}
		repr := taskRepr{
			when: t,
		}
		msg := fmt.Sprintf("Great! a task is scheduled for %s\n now choose what to do", t.Format("02/01/06 15:04 MST (Mon)"))
		b.Send(m.Sender, msg, getAddKeyboard(repr))
	}

	b.Handle("/schedule", newTaskHandler)

	b.Handle(&btnAddTask, func(m *tb.Message) {
		b.Send(m.Sender, newTaskMagic, tb.ForceReply)
	})

	b.Handle(tb.OnText, func(m *tb.Message) {
		if m.ReplyTo.Text != newTaskMagic {
			return
		}
		m.Payload = m.Text
		newTaskHandler(m)
	})
	b.Handle(&btnStatus, apiMiddleware(b, a.Status))
	b.Handle(&btnOn, apiMiddleware(b, a.TurnOn))
	b.Handle(&btnOff, apiMiddleware(b, a.TurnOff))

	b.Handle(addApiOn, addApiHandler(b, addApiOn.Unique))
	b.Handle(addApiOff, addApiHandler(b, addApiOff.Unique))
	b.Handle(addApiStatus, addApiHandler(b, addApiStatus.Unique))

	b.Handle(addRecurring, addRecurringHandler(b, addRecurring.Unique))

	b.Handle(addCancel, func(c *tb.Callback) {
		b.Delete(c.Message)
		b.Respond(c)
	})

	b.Handle(addDone, func(c *tb.Callback) {
		var repr taskRepr
		repr.UnmarshalText([]byte(c.Data))
		if repr.what == "" {
			b.Respond(c, &tb.CallbackResponse{Text: "Not done yet!"})
			return
		}
		var what func()
		var desc string
		for name, f := range addApiButtons {
			if name == repr.what {
				what = apiTaskAdapter(b, c.Sender, f.what)
				desc = f.desc
			}
		}
		task := schedule.Task{
			When:      repr.when,
			What:      what,
			Desc:      desc,
			Recurring: repr.recurring,
		}
		s.Add(task)

		b.Delete(c.Message)
		b.Respond(c, &tb.CallbackResponse{Text: "Task has been added\n"})
		b.Send(c.Sender, "Task has been added\n"+task.String(), menu)

	})

	b.Handle(&btnSchedule, func(m *tb.Message) {
		s.L.Lock()
		defer s.L.Unlock()
		for id, t := range s.Tasks {
			btn := with(delInline, id)
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
