package logging

import "github.com/retail-ai-inc/bean/v2/logging/types"

type Pipeline struct {
	processors []Processor
	sink       Sink
}

func NewPipeline(sink Sink, processors ...Processor) *Pipeline {
	return &Pipeline{
		processors: processors,
		sink:       sink,
	}
}

func (p *Pipeline) Process(entry types.Entry) {
	for _, processor := range p.processors {
		entry = processor.Process(entry)
	}

	err := p.sink.Write(entry)
	if err != nil {
		return
	}
}
