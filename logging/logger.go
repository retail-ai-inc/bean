package logging

import (
	"context"
	"github.com/retail-ai-inc/bean/v2/logging/types"
	"time"
)

type Logger struct {
	traceExtractor TraceExtractor
	pipeline       *Pipeline
}

func New(extractor TraceExtractor, pipeline *Pipeline) *Logger {
	return &Logger{
		traceExtractor: extractor,
		pipeline:       pipeline,
	}
}

func (l *Logger) Info(ctx context.Context, level string, fields map[string]any) {
	l.log(ctx, types.Info, level, fields)
}

func (l *Logger) Error(ctx context.Context, level string, fields map[string]any) {
	l.log(ctx, types.Error, level, fields)
}

func (l *Logger) log(ctx context.Context, severity types.Severity, level string, fields map[string]any) {
	entry := types.Entry{
		Timestamp: time.Now(),
		Severity:  severity,
		Level:     level,
		Fields:    fields,
		Trace:     l.traceExtractor.Extract(ctx),
	}

	l.pipeline.Process(entry)
}
