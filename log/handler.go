package log

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime"
	"time"

	bctx "github.com/retail-ai-inc/bean/v3/context"
)

// LevelCritical is the slog level written as severity `CRITICAL` (used by `Fatal*` / `Panic*`).
const LevelCritical = slog.Level(12)

// severityOf maps a slog level to the `severity` written in the log (Google Cloud Logging compatible).
func severityOf(level slog.Level) Severity {
	switch {
	case level < slog.LevelInfo:
		return Debug
	case level < slog.LevelWarn:
		return Info
	case level < slog.LevelError:
		return Warning
	case level < LevelCritical:
		return Error
	default:
		return Critical
	}
}

// handler is a `slog.Handler` writing through a `Pipeline`, so application logs share the encoding (and the
// `timestamp` / `severity` / `message` / trace fields) with the access log. For every record it also adds:
//   - `id`: the request ID from the context (when the record does not set `id` itself),
//   - the trace ID of the Sentry span in the context (under the runtime platform's trace key),
//   - `file` / `line`: the source location of the log call (when addSource is true).
type handler struct {
	pipeline  *Pipeline
	level     slog.Leveler
	extractor TraceExtractor
	addSource bool

	preAttrs []groupedAttr // attributes added with `WithAttrs`
	groups   []string      // groups opened with `WithGroup`
}

type groupedAttr struct {
	groups []string
	attr   slog.Attr
}

var _ slog.Handler = (*handler)(nil)

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	fields := make(map[string]any, r.NumAttrs()+len(h.preAttrs)+4)

	for _, pa := range h.preAttrs {
		addAttr(fields, pa.groups, pa.attr)
	}
	r.Attrs(func(a slog.Attr) bool {
		addAttr(fields, h.groups, a)
		return true
	})

	if r.Message != "" {
		fields["message"] = r.Message
	}

	if h.addSource && r.PC != 0 {
		frame, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		if frame.File != "" {
			fields["file"] = filepath.Join(filepath.Base(filepath.Dir(frame.File)), filepath.Base(frame.File))
			fields["line"] = frame.Line
		}
	}

	if _, ok := fields["id"]; !ok && ctx != nil {
		if reqID, ok := bctx.GetRequestID(ctx); ok {
			fields["id"] = reqID
		}
	}

	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}

	var trace Trace
	if ctx != nil && h.extractor != nil {
		trace = h.extractor.Extract(ctx)
	}

	return h.pipeline.Process(Entry{
		Timestamp: ts,
		Severity:  severityOf(r.Level),
		Fields:    fields,
		Trace:     trace,
	})
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.clone()
	for _, a := range attrs {
		h2.preAttrs = append(h2.preAttrs, groupedAttr{groups: h.groups, attr: a})
	}
	return h2
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groups = append(append([]string(nil), h.groups...), name)
	return h2
}

func (h *handler) clone() *handler {
	h2 := *h
	h2.preAttrs = append([]groupedAttr(nil), h.preAttrs...)
	return &h2
}

// addAttr adds the attribute to fields under the given groups (nested objects).
func addAttr(fields map[string]any, groups []string, a slog.Attr) {
	a.Value = a.Value.Resolve()

	// Inline groups with an empty key; ignore empty attributes and empty groups (slog handler rules).
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return
		}
		if a.Key != "" {
			groups = append(append([]string(nil), groups...), a.Key)
		}
		for _, ga := range attrs {
			addAttr(fields, groups, ga)
		}
		return
	}
	if a.Key == "" {
		return
	}

	target := fields
	for _, g := range groups {
		next, ok := target[g].(map[string]any)
		if !ok {
			next = map[string]any{}
			target[g] = next
		}
		target = next
	}
	target[a.Key] = attrValue(a.Value)
}

func attrValue(v slog.Value) any {
	switch v.Kind() {
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return err.Error()
		}
		return v.Any()
	default:
		return v.Any()
	}
}
