package log

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

func (p *Pipeline) Process(entry Entry) {
	if entry.Fields != nil {
		for _, processor := range p.processors {
			entry = processor.Process(entry)
		}
	}

	_ = p.sink.Write(entry)
}
