package sinks

import (
	"encoding/json"
	"fmt"
	"github.com/retail-ai-inc/bean/v2/logging/types"
)

type StdoutSink struct{}

func NewStdoutSink() *StdoutSink {
	return &StdoutSink{}
}

func (s *StdoutSink) Write(entry types.Entry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
