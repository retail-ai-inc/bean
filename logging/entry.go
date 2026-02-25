package logging

import (
	"context"
	"time"
)

type Entry struct {
	Timestamp    time.Time
	Severity     string
	TraceID      string
	Method       string
	URL          string
	Status       int
	Latency      time.Duration
	Error        string
	RequestBody  []byte
	ResponseBody []byte
	Context      context.Context
}
