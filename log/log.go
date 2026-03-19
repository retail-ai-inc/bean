package log

import (
	"context"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/config"
	"github.com/retail-ai-inc/bean/v2/helpers"
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
	AccessLogger
}

type AccessLogger interface {
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
	accessLogPath string
	maskFields    []string
}

type LoggerOptions func(*Config)

func WithAccessLogPath(accessLogPath string) LoggerOptions {
	return func(c *Config) {
		c.accessLogPath = accessLogPath
	}
}

func WithMaskFields(maskFields []string) LoggerOptions {
	return func(c *Config) {
		c.maskFields = maskFields
	}
}

func NewLogger(elogger echo.Logger, options ...LoggerOptions) (*logger, error) {
	config := &Config{
		maskFields: []string{},
	}

	for _, option := range options {
		option(config)
	}

	output := elogger.Output()
	if config.accessLogPath != "" {
		file, err := helpers.OpenFile(config.accessLogPath)
		if err != nil {
			return nil, err
		}
		output = file
	}

	sink, err := NewSink(output)
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
		blogger, err = NewLogger(logger,
			WithMaskFields(config.Bean.AccessLog.BodyDumpMaskParam),
			WithAccessLogPath(config.Bean.AccessLog.Path),
		)
		if err != nil {
			panic(err)
		}
	})

	return blogger
}

func Logger() BeanLogger {
	return blogger
}
