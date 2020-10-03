package schedule

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

func TomorrowAt(hour, minute int) time.Time {
	t := time.Now()
	return time.Date(t.Year(), t.Month(), t.Day()+1, hour, minute, 0, 0, t.Location())
}

func Next(hour, minute int) time.Time {
	t := time.Now()
	n := time.Date(t.Year(), t.Month(), t.Day(), hour, minute, 0, 0, t.Location())
	if t.After(n) {
		n = n.Add(24 * time.Hour)
	}
	return n
}

type Task struct {
	When      time.Time
	What      func()
	Desc      string
	Recurring bool
	//TODO: days []time.Weekday
	t *time.Timer
}

func (t Task) String() string {
	format := "Task: %s\nAt: %s"
	if t.Recurring {
		format = "Recurring " + format
	}
	return fmt.Sprintf(format, t.Desc, t.When.In(time.Local).Format("02/01 15:04 (Mon)"))
}

type Schedule struct {
	Tasks map[string]*Task
	L     sync.Mutex
}

func NewSchedule() *Schedule {
	return &Schedule{
		make(map[string]*Task),
		sync.Mutex{},
	}
}

func genID() string {
	var uuid [6]byte
	_, _ = rand.Read(uuid[:])
	return base64.RawStdEncoding.EncodeToString(uuid[:])
}

func (s *Schedule) Add(task Task) {
	s.L.Lock()
	defer s.L.Unlock()
	d := task.When.Sub(time.Now())
	tid := genID()
	s.Tasks[tid] = &task
	task.t = time.AfterFunc(d, func() {
		go task.What()
		s.Remove(tid)
		if task.Recurring {
			h, m, _ := task.When.Clock()
			task.When = TomorrowAt(h, m)
			s.Add(task)
		}
	})
}

func (s *Schedule) Remove(taskId string) bool {
	s.L.Lock()
	defer s.L.Unlock()
	t, ok := s.Tasks[taskId]
	if !ok {
		return false
	}
	delete(s.Tasks, taskId)
	return t.t.Stop()
}
