package beanq

import (
	"github.com/retail-ai-inc/bean/helpers/beanq/stringx"
	"time"
)

type Task struct {
	id      string
	name    string
	payload []byte
	addTime time.Time
}

func NewTask(name string, payload []byte) *Task {
	return &Task{
		name:    name,
		payload: payload,
	}
}
func (t Task) Name() string {
	return t.name
}

func (t Task) Payload() string {
	return stringx.ByteToString(t.payload)
}

func (t Task) Id() string {
	return t.id
}
func (t Task) AddTime() time.Time {
	return t.addTime
}
