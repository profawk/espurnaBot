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
	menu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnStatus = menu.Text("ℹ Status")
	btnOn     = menu.Text("⚡ Power On")
	btnOff    = menu.Text("🔌 Power Off")
	//btnSchedule = menu.Text("⚙ Schedule")
)

func init() {
	menu.Reply(
		menu.Row(btnStatus),
		menu.Row(btnOn, btnOff),
		//menu.Row(btnSchedule),
	)
}

func main() {
	a := api.NewAPI(config.Espurna.ApiKey, config.Espurna.Hostname, config.Espurna.Relay)

	b, err := tb.NewBot(tb.Settings{
		Token:  config.BotToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
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

	apiMiddleware := func(apiCall func() api.State) func(m *tb.Message) {
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

	b.Handle(&btnStatus, apiMiddleware(a.Status))

	b.Handle(&btnOn, apiMiddleware(func() api.State {
		return a.Turn(api.On)
	}))

	b.Handle(&btnOff, apiMiddleware(func() api.State {
		return a.Turn(api.Off)
	}))

	//TODO:
	// add schedules via command and delete with data for constant button "/f"
	b.Start()
}
