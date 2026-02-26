package types

import (
	"time"
)

type Entry struct {
	Timestamp time.Time
	Severity  Severity
	Level     string
	Fields    map[string]any
	Trace     Trace
	Resource  Resource
}
