package log

import (
	"encoding/json"
	"io"
	"strings"
	"time"
)

type Sink interface {
	Write(entry Entry) error
}

type sink struct {
	writer       io.Writer
	payloadTrace string
}

func NewSink(writer io.Writer, payloadTrace string) (*sink, error) {
	gs := &sink{
		writer:       writer,
		payloadTrace: strings.TrimSpace(payloadTrace),
	}

	return gs, nil
}

func (g *sink) Write(e Entry) error {
	payload := map[string]any{
		"timestamp": e.Timestamp.Format(time.RFC3339Nano),
		"severity":  e.Severity,
		"level":     e.Level,
	}

	for k, v := range e.Fields {
		payload[k] = v
	}

	if e.Trace.TraceID != "" {
		payload[g.payloadTrace] = e.Trace.TraceID
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = g.writer.Write(append(b, '\n'))
	if err != nil {
		return err
	}

	return nil
}
