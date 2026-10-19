package async

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/config"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/retail-ai-inc/bean/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncFailureLogUsesIDKey(t *testing.T) {

	originalConf := config.Bean
	config.Bean = &config.Config{}
	t.Cleanup(func() { config.Bean = originalConf })

	logger := log.Init(echo.New().Logger)
	originalOutput := logger.Output()
	t.Cleanup(func() { logger.SetOutput(originalOutput) })

	tests := []struct {
		name        string
		ctx         context.Context
		task        TaskWithCtx
		wantMessage string
		wantID      string
	}{
		{
			name:        "task error with request id",
			ctx:         bctx.SetRequestID(context.Background(), "req-1"),
			task:        func(context.Context) error { return errors.New("boom") },
			wantMessage: "Async task failed.",
			wantID:      "req-1",
		},
		{
			name:        "task error without request id",
			ctx:         context.Background(),
			task:        func(context.Context) error { return errors.New("boom") },
			wantMessage: "Async task failed.",
		},
		{
			name:        "task panic with request id",
			ctx:         bctx.SetRequestID(context.Background(), "req-2"),
			task:        func(context.Context) error { panic("boom") },
			wantMessage: "Recovered panic from async task.",
			wantID:      "req-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &syncBuffer{}
			logger.SetOutput(buf)

			newTask(tt.ctx, tt.task, nil)()

			entry := findBoomEntry(t, buf.String())
			assert.Equal(t, tt.wantMessage, entry["message"])
			assert.NotContains(t, entry, "request_id")
			if tt.wantID == "" {
				assert.NotContains(t, entry, "id")
			} else {
				assert.Equal(t, tt.wantID, entry["id"])
			}
		})
	}
}

// syncBuffer guards writes because goroutines left by other tests share the global logger.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func findBoomEntry(t *testing.T, out string) map[string]any {
	t.Helper()

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry["error"] == "boom" || entry["cause"] == "boom" {
			return entry
		}
	}
	require.FailNow(t, "async failure log not found", out)

	return nil
}
