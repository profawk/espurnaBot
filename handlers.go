package main

import (
	"fmt"
	"github.com/profawk/espurnaBot/api"
	"github.com/profawk/espurnaBot/schedule"
	tb "gopkg.in/tucnak/telebot.v2"
	"log"
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

func sendApiMessage(b *tb.Bot, dest tb.Recipient, tpl string, apiCall api.ApiCall) {
	s, err := apiCall()
	var msg string
	if err != nil {
		log.Print("sendApiMessage tpl: ", tpl, "err: ", err)
		msg = fmt.Sprintf("error occured %v", err)
	} else {
		msg = fmt.Sprintf(tpl, api.StateNames[s])
	}
	_, err = b.Send(dest, msg, menu)
	if err != nil {
		log.Print("status: ", err)
	}
}
func apiMiddleware(b *tb.Bot, apiCall api.ApiCall) func(m *tb.Message) {
	return func(m *tb.Message) {
		sendApiMessage(b, m.Sender, "The relay is %s", apiCall)
	}
}

func setHandlers(b *tb.Bot, a *api.Api, s *schedule.Schedule) {
	b.Handle("/start", func(m *tb.Message) {
		b.Send(m.Sender, "Here is the menu", menu)
	})

	b.Handle("/help", func(m *tb.Message) {
		b.Send(m.Sender, "/start to start\n/schedule to add a schedule - /schedule 20/12 10:25", menu)
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

	addApiButtons := map[string]struct {
		what api.ApiCall
		desc string
	}{
		addApiStatus.Unique: {a.Status, "Get status"},
		addApiOn.Unique:     {a.TurnOn, "Turn on relay"},
		addApiOff.Unique:    {a.TurnOff, "Turn off relay"},
	}

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

}
