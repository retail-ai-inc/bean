package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	bctx "github.com/retail-ai-inc/bean/v3/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T, opts ...LoggerOptions) (*BeanLogger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	l, err := NewLogger(&buf, opts...)
	require.NoError(t, err)
	return l, &buf
}

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m), line)
		out = append(out, m)
	}
	return out
}

func lastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	ls := lines(t, buf)
	require.NotEmpty(t, ls, "no log written")
	return ls[len(ls)-1]
}

func TestHandler_Format(t *testing.T) {
	l, buf := newTestLogger(t)

	l.Info("hello", "user_id", 42, "err", errors.New("boom"), "took", 1500*time.Millisecond)

	m := lastLine(t, buf)
	assert.Equal(t, "INFO", m["severity"])
	assert.Equal(t, "hello", m["message"])
	assert.Equal(t, float64(42), m["user_id"])
	assert.Equal(t, "boom", m["err"], "errors are written as their message")
	assert.Equal(t, "1.5s", m["took"])
	assert.NotContains(t, m, "level", "application logs have no access log category")
	assert.Equal(t, "log/handler_test.go", m["file"], "the source location is the caller")
	assert.NotZero(t, m["line"])

	ts, ok := m["timestamp"].(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339Nano, ts)
	assert.NoError(t, err)
}

func TestHandler_Severity(t *testing.T) {
	l, buf := newTestLogger(t)
	l.Debug("d")
	l.Warn("w")
	l.Error("e")
	l.Log(context.Background(), LevelCritical, "c")

	var got []any
	for _, m := range lines(t, buf) {
		got = append(got, m["severity"])
	}
	assert.Equal(t, []any{"DEBUG", "WARNING", "ERROR", "CRITICAL"}, got)
}

func TestHandler_ContextFields(t *testing.T) {
	l, buf := newTestLogger(t, WithRuntimePlatform("gcp"))

	ctx := bctx.SetRequestID(context.Background(), "req-1")
	span := sentry.StartSpan(ctx, "test")
	defer span.Finish()

	l.InfoContext(span.Context(), "with context")
	m := lastLine(t, buf)
	assert.Equal(t, "req-1", m["id"])
	assert.Equal(t, span.TraceID.String(), m["logging.googleapis.com/trace"])

	l.InfoContext(ctx, "explicit id wins", "id", "custom")
	assert.Equal(t, "custom", lastLine(t, buf)["id"])

	l.Info("no context")
	assert.NotContains(t, lastLine(t, buf), "id")
}

func TestHandler_AttrsAndGroups(t *testing.T) {
	l, buf := newTestLogger(t)

	l.With("service", "api").WithGroup("req").With("method", "GET").
		Info("grouped", "path", "/users", slog.Group("", slog.String("inlined", "yes")), slog.Group("empty"))

	m := lastLine(t, buf)
	assert.Equal(t, "api", m["service"])
	assert.Equal(t, map[string]any{"method": "GET", "path": "/users", "inlined": "yes"}, m["req"])
	assert.NotContains(t, m, "empty")
}

func TestHandler_Level(t *testing.T) {
	l, buf := newTestLogger(t, WithLevel(slog.LevelInfo))

	l.Debug("hidden")
	assert.Empty(t, buf.String())

	l.SetLevel(slog.LevelDebug)
	l.Debug("shown")
	assert.Equal(t, "shown", lastLine(t, buf)["message"])
	assert.Equal(t, slog.LevelDebug, l.Level())
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{
		"": slog.LevelDebug, "debug": slog.LevelDebug, "INFO": slog.LevelInfo,
		"warn": slog.LevelWarn, "Warning": slog.LevelWarn, "error": slog.LevelError,
	} {
		got, err := ParseLevel(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	_, err := ParseLevel("verbose")
	assert.ErrorContains(t, err, "debugLogLevel")
}

func TestCompatMethods(t *testing.T) {
	l, buf := newTestLogger(t)

	l.Debugf("user %d logged in", 7)
	m := lastLine(t, buf)
	assert.Equal(t, "DEBUG", m["severity"])
	assert.Equal(t, "user 7 logged in", m["message"])
	assert.Equal(t, "log/handler_test.go", m["file"], "the source location is the caller of the deprecated method")

	l.Errorj(map[string]any{"message": "failed", "error": "boom"})
	m = lastLine(t, buf)
	assert.Equal(t, "ERROR", m["severity"])
	assert.Equal(t, "failed", m["message"])
	assert.Equal(t, "boom", m["error"])
	assert.Equal(t, "log/handler_test.go", m["file"])

	l.Warnf("w")
	assert.Equal(t, "WARNING", lastLine(t, buf)["severity"])

	assert.PanicsWithValue(t, "bad 1", func() { l.Panicf("bad %d", 1) })
	assert.Equal(t, "CRITICAL", lastLine(t, buf)["severity"])
}

func TestAccessLogSharesFormatAndShutdownKeepsAppLogs(t *testing.T) {
	l, buf := newTestLogger(t)
	SetLogger(l)
	t.Cleanup(func() { SetLogger(nil) })

	l.TraceInfo(bctx.SetRequestID(context.Background(), "req-1"), "ACCESS", map[string]any{"id": "req-1", "uri": "/"})
	m := lastLine(t, buf)
	assert.Equal(t, "INFO", m["severity"])
	assert.Equal(t, "ACCESS", m["level"])
	assert.Contains(t, m, "timestamp")

	require.NoError(t, Shutdown(context.Background()))
	l.Info("after shutdown")
	assert.Equal(t, "after shutdown", lastLine(t, buf)["message"], "application logs are written after Shutdown")
}
