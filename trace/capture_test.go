package trace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	elog "github.com/labstack/gommon/log"
	"github.com/retail-ai-inc/bean/v2/config"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/retail-ai-inc/bean/v2/log"
	"github.com/retail-ai-inc/bean/v2/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogAndSentryCaptureWithRequestID(t *testing.T) {

	originalConf := config.Bean
	config.Bean = &config.Config{}
	t.Cleanup(func() { config.Bean = originalConf })

	logger := log.Init(echo.New().Logger)
	originalLevel := logger.Level()
	originalOutput := logger.Output()
	logger.SetLevel(elog.DEBUG)
	t.Cleanup(func() {
		logger.SetLevel(originalLevel)
		logger.SetOutput(originalOutput)
	})

	tests := []struct {
		name          string
		ctx           func() context.Context
		capture       func(ctx context.Context)
		wantLevel     string
		wantMessage   string
		wantRequestID string
	}{
		{
			name:          "exception with request id",
			ctx:           func() context.Context { return bctx.SetRequestID(context.Background(), "req-1") },
			capture:       func(ctx context.Context) { trace.LogAndSentryCaptureException(ctx, errors.New("boom")) },
			wantLevel:     "ERROR",
			wantMessage:   "boom",
			wantRequestID: "req-1",
		},
		{
			name:        "exception without request id",
			ctx:         context.Background,
			capture:     func(ctx context.Context) { trace.LogAndSentryCaptureException(ctx, errors.New("boom")) },
			wantLevel:   "ERROR",
			wantMessage: "boom",
		},
		{
			name:        "exception with nil context",
			ctx:         func() context.Context { return nil },
			capture:     func(ctx context.Context) { trace.LogAndSentryCaptureException(ctx, errors.New("boom")) },
			wantLevel:   "ERROR",
			wantMessage: "boom",
		},
		{
			name:          "message with request id",
			ctx:           func() context.Context { return bctx.SetRequestID(context.Background(), "req-2") },
			capture:       func(ctx context.Context) { trace.LogAndSentryCaptureMessage(ctx, "hello") },
			wantLevel:     "INFO",
			wantMessage:   "hello",
			wantRequestID: "req-2",
		},
		{
			name:        "message without request id",
			ctx:         context.Background,
			capture:     func(ctx context.Context) { trace.LogAndSentryCaptureMessage(ctx, "hello") },
			wantLevel:   "INFO",
			wantMessage: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger.SetOutput(&buf)

			tt.capture(tt.ctx())

			var entry map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry), buf.String())
			assert.Equal(t, tt.wantLevel, entry["level"])
			assert.Equal(t, tt.wantMessage, entry["message"])
			if tt.wantRequestID == "" {
				assert.NotContains(t, entry, "id")
			} else {
				assert.Equal(t, tt.wantRequestID, entry["id"])
			}
		})
	}
}

func TestSentryCaptureDoesNotLog(t *testing.T) {

	originalConf := config.Bean
	config.Bean = &config.Config{}
	t.Cleanup(func() { config.Bean = originalConf })

	logger := log.Init(echo.New().Logger)
	originalOutput := logger.Output()
	t.Cleanup(func() { logger.SetOutput(originalOutput) })

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	ctx := bctx.SetRequestID(context.Background(), "req-1")
	trace.SentryCaptureException(ctx, errors.New("boom"))
	trace.SentryCaptureMessage(ctx, "hello")
	trace.SentryCaptureExceptionWithEcho(nil, errors.New("boom"))
	trace.SentryCaptureMessageWithEcho(nil, "hello")

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	c := echo.New().NewContext(req, httptest.NewRecorder())
	trace.SentryCaptureExceptionWithEcho(c, errors.New("boom"))
	trace.SentryCaptureMessageWithEcho(c, "hello")

	assert.Empty(t, buf.String())
}
