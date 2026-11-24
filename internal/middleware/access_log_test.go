package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"
	elog "github.com/labstack/gommon/log"
	"github.com/retail-ai-inc/bean/v3/log"
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

// errAfterReader returns data and then a non-EOF error, like a client aborting an upload.
type errAfterReader struct {
	data []byte
	err  error
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestAccessLoggerWithConfig_BodyDumpIsBounded(t *testing.T) {
	const limit = 16
	big := strings.Repeat("a", 1<<20) // 1MB

	tests := []struct {
		name         string
		handler      echo.HandlerFunc
		wantReqBody  string
		wantResBody  string
		wantBytesOut int64
	}{
		{
			name: "handler reads the whole request and writes a large response",
			handler: func(c *echo.Context) error {
				b, err := io.ReadAll(c.Request().Body)
				if err != nil {
					return err
				}
				if string(b) != big {
					return c.String(http.StatusInternalServerError, "request body was altered")
				}
				return c.String(http.StatusOK, big)
			},
			wantReqBody:  big[:limit+1],
			wantResBody:  big[:limit+1],
			wantBytesOut: int64(len(big)),
		},
		{
			name: "handler does not read the request body",
			handler: func(c *echo.Context) error {
				return c.String(http.StatusOK, "ok")
			},
			wantReqBody:  big[:limit+1],
			wantResBody:  "ok",
			wantBytesOut: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &fakeAccessLogger{}
			e := echo.New()
			e.Use(AccessLoggerWithConfig(LoggerConfig{BodyDump: true, Logger: logger, BodyLimit: limit}))
			e.POST("/", tt.handler)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big)))

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			fields := logger.dump(t)
			assert.Equal(t, tt.wantReqBody, fields["request_body"])
			assert.Equal(t, tt.wantResBody, fields["response_body"])
			assert.Equal(t, tt.wantBytesOut, fields["bytes_out"])
			assert.NotContains(t, fields, "request_body_error")
		})
	}
}

func TestAccessLoggerWithConfig_RequestBodyReadError(t *testing.T) {
	logger := &fakeAccessLogger{}
	e := echo.New()
	e.Use(AccessLoggerWithConfig(LoggerConfig{BodyDump: true, Logger: logger, BodyLimit: 4}))

	var handlerErr error
	e.POST("/", func(c *echo.Context) error {
		_, handlerErr = io.ReadAll(c.Request().Body)
		return c.NoContent(http.StatusBadRequest)
	})

	readErr := errors.New("client aborted")
	req := httptest.NewRequest(http.MethodPost, "/", &errAfterReader{data: []byte("0123456789"), err: readErr})
	e.ServeHTTP(httptest.NewRecorder(), req)

	// The handler must see the read error instead of a silently truncated body.
	assert.ErrorIs(t, handlerErr, readErr)

	fields := logger.dump(t)
	assert.Equal(t, "01234", fields["request_body"])
	assert.Equal(t, readErr.Error(), fields["request_body_error"])
	assert.Equal(t, int64(10), fields["bytes_in_read"])
}

func TestAccessLoggerWithConfig_GzipResponseSize(t *testing.T) {
	logger := &fakeAccessLogger{}
	e := echo.New()
	e.Use(AccessLoggerWithConfig(LoggerConfig{BodyDump: true, Logger: logger, BodyLimit: 4}))
	e.GET("/", func(c *echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentEncoding, "gzip")
		return c.Blob(http.StatusOK, "application/octet-stream", make([]byte, 100))
	})

	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	fields := logger.dump(t)
	assert.Equal(t, "[gzip compressed response omitted]", fields["response_body"])
	assert.Equal(t, int64(100), fields["response_body_size"])
}

// The access log output must be the same as when the whole body was buffered: the log pipeline truncates the
// body to the same limit.
func TestAccessLoggerWithConfig_LogOutputIsTruncated(t *testing.T) {
	const limit = 8
	var out bytes.Buffer
	elogger := elog.New("test")
	elogger.SetOutput(&out)
	logger, err := log.NewLogger(elogger, log.WithBodyLimit(limit))
	require.NoError(t, err)

	e := echo.New()
	e.Use(AccessLoggerWithConfig(LoggerConfig{BodyDump: true, Logger: logger, BodyLimit: limit}))
	e.POST("/", func(c *echo.Context) error {
		return c.String(http.StatusOK, "response-body-longer-than-limit")
	})
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("request-body-longer-than-limit")))

	var dump map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m))
		if m["level"] == "DUMP" {
			dump = m
		}
	}
	require.NotNil(t, dump, out.String())
	assert.Equal(t, "request-...(truncated)", dump["request_body"])
	assert.Equal(t, "response...(truncated)", dump["response_body"])
}
