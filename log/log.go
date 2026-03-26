package log

import (
	"context"
	"io"
	"strings"
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
	accessLogPath    string
	maskFields       []string
	runtimePlatform  string
	sinkAsync        bool
	sinkAsyncQueueSz int
}

type LoggerOptions func(*Config)

func WithAccessLogPath(accessLogPath string) LoggerOptions {
	return func(c *Config) { c.accessLogPath = accessLogPath }
}

func WithMaskFields(maskFields []string) LoggerOptions {
	return func(c *Config) { c.maskFields = maskFields }
}

func WithRuntimePlatform(platform string) LoggerOptions {
	return func(c *Config) { c.runtimePlatform = platform }
}

func WithSinkAsync(async bool, queueSize int) LoggerOptions {
	return func(c *Config) {
		c.sinkAsync = async
		c.sinkAsyncQueueSz = queueSize
	}
}

func tracePayloadKey(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "gcp", "google":
		return "logging.googleapis.com/trace"
	case "aws", "amazon", "azure", "microsoft":
		return "trace_id"
	default:
		return "trace"
	}
}

func NewLogger(elogger echo.Logger, options ...LoggerOptions) (*logger, error) {
	cfg := &Config{maskFields: []string{}}

	for _, option := range options {
		option(cfg)
	}

	payloadTrace := tracePayloadKey(cfg.runtimePlatform)

	sinkCfg := SinkConfig{
		Async:     cfg.sinkAsync,
		QueueSize: cfg.sinkAsyncQueueSz,
	}

	var out io.WriteCloser = NopWriteCloser{Writer: elogger.Output()}
	if cfg.accessLogPath != "" {
		file, err := helpers.OpenFile(cfg.accessLogPath)
		if err != nil {
			return nil, err
		}
		out = file
	}

	s, err := NewSink(out, payloadTrace, sinkCfg)
	if err != nil {
		return nil, err
	}

	processors := make([]Processor, 0, 2)
	if len(cfg.maskFields) > 0 {
		processors = append(processors, NewMaskProcessor(cfg.maskFields))
	}
	processors = append(processors, NewRemoveEscapeProcessor())

	return &logger{
		Logger:         elogger,
		traceExtractor: NewSentryExtractor(),
		pipeline:       NewPipeline(s, processors...),
	}, nil
}

func (l *logger) TraceInfo(ctx context.Context, level string, fields map[string]any) {
	l.traceLog(ctx, Info, level, fields)
}

func (l *logger) TraceError(ctx context.Context, level string, fields map[string]any) {
	l.traceLog(ctx, Error, level, fields)
}

func (l *logger) traceLog(ctx context.Context, severity Severity, level string, fields map[string]any) {
	_ = l.pipeline.Process(Entry{
		Timestamp: time.Now(),
		Severity:  severity,
		Level:     level,
		Fields:    fields,
		Trace:     l.traceExtractor.Extract(ctx),
	})
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
			WithRuntimePlatform(config.Bean.AccessLog.RuntimePlatform),
			WithSinkAsync(config.Bean.AccessLog.Async, config.Bean.AccessLog.AsyncQueueSize),
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

func Shutdown(ctx context.Context) error {
	if l, ok := blogger.(*logger); ok && l != nil && l.pipeline != nil {
		return l.pipeline.Close(ctx)
	}
	return nil
}
