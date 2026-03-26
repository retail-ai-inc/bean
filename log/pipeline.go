package log

import "context"

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

func (p *Pipeline) Process(entry Entry) error {
	if entry.Fields != nil {
		for _, processor := range p.processors {
			entry = processor.Process(entry)
		}
	}

	return p.sink.Write(entry)
}

func (p *Pipeline) Close(ctx context.Context) error {
	if c, ok := p.sink.(interface{ Close(context.Context) error }); ok {
		return c.Close(ctx)
	}
	return nil
}
