package sinks

import (
	"encoding/json"
	"fmt"
	"github.com/retail-ai-inc/bean/v2/logging/types"
	"os"
	"time"
)

type Options struct {
	ProjectID  string
	LogType    string
	OutputFile string
}

type GcpSink struct {
	opt  Options
	file *os.File
}

func NewGcpSink(opt Options) (*GcpSink, error) {
	var f *os.File
	var err error

	if opt.OutputFile != "" {
		f, err = os.OpenFile(opt.OutputFile,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
	}

	return &GcpSink{
		opt:  opt,
		file: f,
	}, nil
}

func (g *GcpSink) Write(e types.Entry) error {
	payload := map[string]any{
		"timestamp": e.Timestamp.Format(time.RFC3339Nano),
		"severity":  e.Severity,
		"level":     e.Level,
		"type":      g.opt.LogType,
	}

	for k, v := range e.Fields {
		payload[k] = v
	}

	if e.Trace.TraceID != "" {
		payload["logging.googleapis.com/trace"] =
			fmt.Sprintf("projects/%s/traces/%s",
				g.opt.ProjectID,
				e.Trace.TraceID,
			)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if g.file != nil {
		_, err = g.file.Write(append(b, '\n'))
		return err
	}

	fmt.Println(string(b))
	return nil
}
