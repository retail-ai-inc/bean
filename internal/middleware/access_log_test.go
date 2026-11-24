package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedEntry struct {
	level  string
	fields map[string]any
}

type fakeAccessLogger struct {
	mu      sync.Mutex
	entries []recordedEntry
}

func (l *fakeAccessLogger) TraceInfo(_ context.Context, level string, fields map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, recordedEntry{level: level, fields: fields})
}

func (l *fakeAccessLogger) TraceError(ctx context.Context, level string, fields map[string]any) {
	l.TraceInfo(ctx, level, fields)
}

func (l *fakeAccessLogger) dump(t *testing.T) map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e.level == "DUMP" {
			return e.fields
		}
	}
	t.Fatal("no DUMP entry recorded")
	return nil
}

func TestAccessLoggerWithConfig_BodyDump(t *testing.T) {
	logger := &fakeAccessLogger{}
	e := echo.New()
	e.Use(AccessLoggerWithConfig(LoggerConfig{BodyDump: true, Logger: logger}))
	e.POST("/ok", func(c *echo.Context) error {
		return c.String(http.StatusCreated, "hello")
	})

	req := httptest.NewRequest(http.MethodPost, "/ok", strings.NewReader(`{"a":1}`))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())

	fields := logger.dump(t)
	assert.Equal(t, http.StatusCreated, fields["status"])
	assert.Equal(t, `{"a":1}`, fields["request_body"])
	assert.Equal(t, "hello", fields["response_body"])
	assert.Equal(t, int64(len("hello")), fields["bytes_out"])
}

func TestAccessLoggerWithConfig_HandlerError(t *testing.T) {
	logger := &fakeAccessLogger{}
	e := echo.New()
	e.Use(AccessLoggerWithConfig(LoggerConfig{BodyDump: true, Logger: logger}))
	e.GET("/err", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "bad")
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// The error response must be written exactly once, by the global error handler.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 1, strings.Count(rec.Body.String(), "bad"))

	fields := logger.dump(t)
	assert.Equal(t, http.StatusBadRequest, fields["status"])
	require.Contains(t, fields, "error")
	assert.Contains(t, fields["response_body"], "bad")
}
