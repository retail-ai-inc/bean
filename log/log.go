package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/retail-ai-inc/bean/v3/helpers"
)

type Severity string

const (
	Debug    Severity = "DEBUG"
	Info     Severity = "INFO"
	Warning  Severity = "WARNING"
	Error    Severity = "ERROR"
	Critical Severity = "CRITICAL"
)

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

// BeanLogger is bean's logger.
//
//   - It embeds `*slog.Logger` for application logs: `Debug`, `Info`, `Warn`, `Error` and their `...Context`
//     variants. Passing the request context (e.g. `InfoContext(ctx, ...)`) adds the request ID and trace ID.
//     The same `*slog.Logger` is used as echo's logger, so `c.Logger()` writes the same format.
//   - `TraceInfo` / `TraceError` write the access log (masked, truncated, optionally asynchronous).
//   - `Debugf`, `Errorj`, ... are kept for compatibility with bean v2 and are deprecated.
//
// Every line is a JSON object with `timestamp`, `severity` and `message` (Google Cloud Logging compatible).
type BeanLogger struct {
	*slog.Logger
	level     *slog.LevelVar
	access    *Pipeline
	extractor TraceExtractor
}

var _ AccessLogger = (*BeanLogger)(nil)

type Config struct {
	accessLogPath    string
	maskFields       []string
	runtimePlatform  string
	sinkAsync        bool
	sinkAsyncQueueSz int
	bodyLimit        int
	level            slog.Level
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

func WithBodyLimit(bodyLimit int) LoggerOptions {
	return func(c *Config) { c.bodyLimit = bodyLimit }
}

// WithLevel sets the minimum level of the application logs (default: debug). It does not affect the access log.
func WithLevel(level slog.Level) LoggerOptions {
	return func(c *Config) { c.level = level }
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

// NewLogger creates a logger writing application logs to w. The access log is written to the access log path
// if set (`WithAccessLogPath`), otherwise to w as well.
func NewLogger(w io.Writer, options ...LoggerOptions) (*BeanLogger, error) {
	cfg := &Config{maskFields: []string{}, level: slog.LevelDebug}
	for _, option := range options {
		option(cfg)
	}

	payloadTrace := tracePayloadKey(cfg.runtimePlatform)
	extractor := NewSentryExtractor()

	// Access log: truncate -> mask -> remove escape -> (async) sink.
	var accessOut io.WriteCloser = NopWriteCloser{Writer: w}
	if cfg.accessLogPath != "" {
		file, err := helpers.OpenFile(cfg.accessLogPath)
		if err != nil {
			return nil, err
		}
		accessOut = file
	}
	accessSink, err := NewSink(accessOut, payloadTrace, SinkConfig{
		Async:     cfg.sinkAsync,
		QueueSize: cfg.sinkAsyncQueueSz,
		BodyLimit: cfg.bodyLimit,
	})
	if err != nil {
		return nil, err
	}
	processors := make([]Processor, 0, 3)
	processors = append(processors, NewTruncateBodyProcessor(cfg.bodyLimit))
	if len(cfg.maskFields) > 0 {
		processors = append(processors, NewMaskProcessor(cfg.maskFields))
	}
	processors = append(processors, NewRemoveEscapeProcessor())

	// Application logs: synchronous sink, same encoding as the access log.
	appSink, err := NewSink(NopWriteCloser{Writer: w}, payloadTrace, SinkConfig{})
	if err != nil {
		return nil, err
	}

	level := new(slog.LevelVar)
	level.Set(cfg.level)

	return &BeanLogger{
		Logger: slog.New(&handler{
			pipeline:  NewPipeline(appSink),
			level:     level,
			extractor: extractor,
			addSource: true,
		}),
		level:     level,
		access:    NewPipeline(accessSink, processors...),
		extractor: extractor,
	}, nil
}

// SetLevel changes the minimum level of the application logs at runtime.
func (l *BeanLogger) SetLevel(level slog.Level) { l.level.Set(level) }

// Level returns the minimum level of the application logs.
func (l *BeanLogger) Level() slog.Level { return l.level.Level() }

func (l *BeanLogger) TraceInfo(ctx context.Context, level string, fields map[string]any) {
	l.traceLog(ctx, Info, level, fields)
}

func (l *BeanLogger) TraceError(ctx context.Context, level string, fields map[string]any) {
	l.traceLog(ctx, Error, level, fields)
}

func (l *BeanLogger) traceLog(ctx context.Context, severity Severity, level string, fields map[string]any) {
	var trace Trace
	if ctx != nil {
		trace = l.extractor.Extract(ctx)
	}
	_ = l.access.Process(Entry{
		Timestamp: time.Now(),
		Severity:  severity,
		Level:     level,
		Fields:    fields,
		Trace:     trace,
	})
}

// ---- bean v2 compatible methods (deprecated) ----

// Deprecated: use `Info` (slog style: `Info("message", "key", value)`).
func (l *BeanLogger) Print(i ...any) { l.compat(slog.LevelInfo, fmt.Sprint(i...)) }

// Deprecated: use `Info` (slog style: `Info("message", "key", value)`).
func (l *BeanLogger) Printf(format string, args ...any) {
	l.compat(slog.LevelInfo, fmt.Sprintf(format, args...))
}

// Deprecated: use `Info` (slog style: `Info("message", "key", value)`).
func (l *BeanLogger) Printj(j map[string]any) { l.compatJSON(slog.LevelInfo, j) }

// Deprecated: use `Debug` (slog style: `Debug("message", "key", value)`).
func (l *BeanLogger) Debugf(format string, args ...any) {
	l.compat(slog.LevelDebug, fmt.Sprintf(format, args...))
}

// Deprecated: use `Debug` (slog style: `Debug("message", "key", value)`).
func (l *BeanLogger) Debugj(j map[string]any) { l.compatJSON(slog.LevelDebug, j) }

// Deprecated: use `Info` (slog style: `Info("message", "key", value)`).
func (l *BeanLogger) Infof(format string, args ...any) {
	l.compat(slog.LevelInfo, fmt.Sprintf(format, args...))
}

// Deprecated: use `Info` (slog style: `Info("message", "key", value)`).
func (l *BeanLogger) Infoj(j map[string]any) { l.compatJSON(slog.LevelInfo, j) }

// Deprecated: use `Warn` (slog style: `Warn("message", "key", value)`).
func (l *BeanLogger) Warnf(format string, args ...any) {
	l.compat(slog.LevelWarn, fmt.Sprintf(format, args...))
}

// Deprecated: use `Warn` (slog style: `Warn("message", "key", value)`).
func (l *BeanLogger) Warnj(j map[string]any) { l.compatJSON(slog.LevelWarn, j) }

// Deprecated: use `Error` (slog style: `Error("message", "error", err)`).
func (l *BeanLogger) Errorf(format string, args ...any) {
	l.compat(slog.LevelError, fmt.Sprintf(format, args...))
}

// Deprecated: use `Error` (slog style: `Error("message", "error", err)`).
func (l *BeanLogger) Errorj(j map[string]any) { l.compatJSON(slog.LevelError, j) }

// Deprecated: log with `Error` and return an error / exit explicitly.
func (l *BeanLogger) Fatal(i ...any) { l.compat(LevelCritical, fmt.Sprint(i...)); l.exit() }

// Deprecated: log with `Error` and return an error / exit explicitly.
func (l *BeanLogger) Fatalf(format string, args ...any) {
	l.compat(LevelCritical, fmt.Sprintf(format, args...))
	l.exit()
}

// Deprecated: log with `Error` and return an error / exit explicitly.
func (l *BeanLogger) Fatalj(j map[string]any) { l.compatJSON(LevelCritical, j); l.exit() }

// Deprecated: log with `Error` and panic explicitly.
func (l *BeanLogger) Panic(i ...any) {
	msg := fmt.Sprint(i...)
	l.compat(LevelCritical, msg)
	panic(msg)
}

// Deprecated: log with `Error` and panic explicitly.
func (l *BeanLogger) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.compat(LevelCritical, msg)
	panic(msg)
}

// Deprecated: log with `Error` and panic explicitly.
func (l *BeanLogger) Panicj(j map[string]any) {
	l.compatJSON(LevelCritical, j)
	panic(fmt.Sprint(j))
}

// compat logs with the source location of the caller of the deprecated method.
func (l *BeanLogger) compat(level slog.Level, msg string, attrs ...slog.Attr) {
	ctx := context.Background()
	if !l.Handler().Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip runtime.Callers, compat, the deprecated method
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = l.Handler().Handle(ctx, r)
}

// compatJSON logs j: the `message` key is the log message, the other keys are attributes.
func (l *BeanLogger) compatJSON(level slog.Level, j map[string]any) {
	ctx := context.Background()
	if !l.Handler().Enabled(ctx, level) {
		return
	}
	var msg string
	attrs := make([]slog.Attr, 0, len(j))
	for k, v := range j {
		if k == "message" {
			msg = fmt.Sprint(v)
			continue
		}
		attrs = append(attrs, slog.Any(k, v))
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip runtime.Callers, compatJSON, the deprecated method
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = l.Handler().Handle(ctx, r)
}

func (l *BeanLogger) exit() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = l.access.Close(ctx) // flush the access log
	os.Exit(1)
}

// ---- global logger ----

// current holds the global bean logger returned by `Logger()`.
var current atomic.Pointer[BeanLogger]

// Init creates the bean logger from `config.Bean` writing application logs to w (and the access log to
// `accessLog.path`, or w if empty), and sets it as the global logger returned by `Logger()`.
// Calling it again replaces the global logger.
func Init(w io.Writer) (*BeanLogger, error) {
	level, err := ParseLevel(config.Bean.DebugLogLevel)
	if err != nil {
		return nil, err
	}

	l, err := NewLogger(w,
		WithLevel(level),
		WithMaskFields(config.Bean.AccessLog.MaskFields),
		WithAccessLogPath(config.Bean.AccessLog.Path),
		WithRuntimePlatform(config.Bean.AccessLog.RuntimePlatform),
		WithSinkAsync(config.Bean.AccessLog.Async, config.Bean.AccessLog.AsyncQueueSize),
		WithBodyLimit(config.Bean.AccessLog.BodyLimit),
	)
	if err != nil {
		return nil, err
	}
	SetLogger(l)
	return l, nil
}

// ParseLevel parses `debug`, `info`, `warn` (or `warning`) and `error` case-insensitively. Empty means debug.
func ParseLevel(s string) (slog.Level, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "":
		return slog.LevelDebug, nil
	case "warning":
		return slog.LevelWarn, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return 0, fmt.Errorf("invalid debugLogLevel %q: %w", s, err)
	}
	return level, nil
}

// SetLogger sets the global logger returned by `Logger()`.
func SetLogger(l *BeanLogger) {
	current.Store(l)
}

// Logger returns the global bean logger, or nil if it is not initialized.
func Logger() *BeanLogger {
	return current.Load()
}

// Shutdown flushes and closes the global logger's access log sink. Application logs can still be written.
func Shutdown(ctx context.Context) error {
	if l := Logger(); l != nil && l.access != nil {
		return l.access.Close(ctx)
	}
	return nil
}
