package log

import (
	"context"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/config"
)

type Severity string

const (
	Debug    Severity = "DEBUG"
	Info     Severity = "INFO"
	Warning  Severity = "WARNING"
	Error    Severity = "ERROR"
	Critical Severity = "CRITICAL"
)

type BeanLogger interface {
	echo.Logger
	TraceInfo(ctx context.Context, level string, fields map[string]any)
	TraceError(ctx context.Context, level string, fields map[string]any)
}

type Trace struct {
	TraceID string
	SpanID  string
}

type Entry struct {
	Timestamp time.Time
	Severity  Severity
	Level     string
	Fields    map[string]any
	Trace     Trace
}

type logger struct {
	echo.Logger
	traceExtractor TraceExtractor
	pipeline       *Pipeline
}

type Config struct {
	projectID  string
	maskFields []string
}

type LoggerOptions func(*Config)

func WithProjectID(projectID string) LoggerOptions {
	return func(c *Config) {
		c.projectID = projectID
	}
}

func WithMaskFields(maskFields []string) LoggerOptions {
	return func(c *Config) {
		c.maskFields = maskFields
	}
}

func NewLogger(elogger echo.Logger, options ...LoggerOptions) (*logger, error) {
	config := &Config{
		projectID:  "project-id",
		maskFields: []string{},
	}

	for _, option := range options {
		option(config)
	}

	sink, err := NewSink(elogger.Output(), config.projectID)
	if err != nil {
		return nil, err
	}

	sentryExtractor := NewSentryExtractor()

	pipeline := NewPipeline(sink, NewMaskProcessor(config.maskFields), NewRemoveEscapeProcessor())

	return &logger{
		Logger:         elogger,
		traceExtractor: sentryExtractor,
		pipeline:       pipeline,
	}, nil
}

func (l *logger) TraceInfo(ctx context.Context, level string, fields map[string]any) {
	l.traceLog(ctx, Info, level, fields)
}

func (l *logger) TraceError(ctx context.Context, level string, fields map[string]any) {
	l.traceLog(ctx, Error, level, fields)
}

func (l *logger) traceLog(ctx context.Context, severity Severity, level string, fields map[string]any) {
	entry := Entry{
		Timestamp: time.Now(),
		Severity:  severity,
		Level:     level,
		Fields:    fields,
		Trace:     l.traceExtractor.Extract(ctx),
	}

	l.pipeline.Process(entry)
}

var (
	blogger BeanLogger
	once    sync.Once
)

func Init(logger echo.Logger) BeanLogger {
	once.Do(func() {
		var err error
		blogger, err = NewLogger(logger, WithProjectID(config.Bean.Sentry.ProjectID))
		if err != nil {
			panic(err)
		}
	})

	return blogger
}

func Logger() BeanLogger {
	return blogger
}
