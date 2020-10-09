package bot

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/profawk/espurnaBot/schedule"
	tb "gopkg.in/tucnak/telebot.v2"
	"time"
)

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}

// map[time.Duration]string is the obvious way to go, however go puts two obstacles
// 1. time.Duration cannot be parsed by json (will be fixed in go 2
// 2. json will not allow Duration to be the key in a map :(
type Triggers struct {
	On  map[string][]Duration
	Off map[string][]Duration
}

var triggers Triggers
var s *schedule.Schedule

func init() {
	triggers = Triggers{
		make(map[string][]Duration),
		make(map[string][]Duration),
	}
	s = schedule.NewSchedule()
}

func AddTriggers(add Triggers) {
	for d, m := range add.On {
		triggers.On[d] = m
	}
	for d, m := range add.Off {
		triggers.Off[d] = m
	}
}

func onTrigger(b *tb.Bot, dest tb.Recipient) {
	trigger(b, dest, "on")
}

func offTrigger(b *tb.Bot, dest tb.Recipient) {
	trigger(b, dest, "off")
}

func trigger(b *tb.Bot, dest tb.Recipient, edge string) {
	s.L.Lock()
	for tid, t := range s.Tasks {
		if t.Desc != edge {
			s.RemoveNoLock(tid)
		}
	}
	s.L.Unlock()

	t := triggers.On
	if edge != "on" {
		t = triggers.Off
	}
	n := time.Now()
	for m, ds := range t {
		for _, d := range ds {
			s.Add(schedule.Task{
				When: n.Add(d.Duration),
				What: func(d time.Duration) func() {
					return func() { b.Send(dest, fmt.Sprintf(m, d)) }
				}(d.Duration),
				Desc:      edge,
				Recurring: false,
			})
		}
	}

}
