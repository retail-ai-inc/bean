package log

import (
	"encoding/json"
	"io"
	"time"
)

type Sink interface {
	Write(entry Entry) error
}

type sink struct {
	writer    io.Writer
}

func NewSink(writer io.Writer) (*sink, error) {
	gs := &sink{
		writer:    writer,
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
		payload["logging.googleapis.com/trace"] = e.Trace.TraceID
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
