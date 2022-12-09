package beanq

import (
	"github.com/retail-ai-inc/bean/helpers/beanq/stringx"
	"time"
)

type Message struct {
	Id      string
	Stream  string
	Payload []byte
}

type Task struct {
	id          string    `json:"id"`
	name        string    `json:"name"`
	payload     []byte    `json:"payload"`
	addTime     string    `json:"addTime"`
	executeTime time.Time `json:"executeTime"`
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
func (t Task) AddTime() string {
	return t.addTime
}
