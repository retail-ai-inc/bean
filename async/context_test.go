package async

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"
	"github.com/retail-ai-inc/bean/v3/config"
	bctx "github.com/retail-ai-inc/bean/v3/context"
	"github.com/retail-ai-inc/bean/v3/internal/gopool"
	"github.com/retail-ai-inc/bean/v3/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taskContext struct {
	requestID string
	hub       *sentry.Hub
	err       error // ctx.Err() when the task runs
}

// setupSentryConfig turns Sentry on for the test. The tasks of other tests read the global config, so it is only
// changed once the default pool is idle.
func setupSentryConfig(t *testing.T) {
	t.Helper()
	waitDefaultPoolIdle()
	prev := config.Bean
	config.Bean = &config.Config{Sentry: config.Sentry{On: true}}
	t.Cleanup(func() {
		waitDefaultPoolIdle()
		config.Bean = prev
	})
	_, _ = log.Init(io.Discard)
}

func waitDefaultPoolIdle() {
	for gopool.GetDefaultPool().Running() > 0 {
		time.Sleep(time.Millisecond)
	}
}

// requestContext returns a canceled request context, like after the handler returned, with a request ID and a
// Sentry hub.
func requestContext(t *testing.T) (context.Context, *sentry.Hub) {
	t.Helper()
	hub := sentry.CurrentHub().Clone()
	ctx, cancel := context.WithCancel(bctx.SetRequestID(context.Background(), "req-1"))
	ctx = sentry.SetHubOnContext(ctx, hub)
	cancel()
	return ctx, hub
}

func capture(done chan<- taskContext) func(ctx context.Context) {
	return func(ctx context.Context) {
		id, _ := bctx.GetRequestID(ctx)
		done <- taskContext{requestID: id, hub: sentry.GetHubFromContext(ctx), err: ctx.Err()}
	}
}

func TestExecuteWithContext_KeepsRequestContext(t *testing.T) {
	setupSentryConfig(t)
	reqCtx, reqHub := requestContext(t)

	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/orders", nil).WithContext(reqCtx), httptest.NewRecorder())

	done := make(chan taskContext, 1)
	ExecuteWithContext(capture(done), c)
	got := <-done

	assert.Equal(t, "req-1", got.requestID)
	require.NotNil(t, got.hub)
	assert.NotSame(t, reqHub, got.hub, "the task must use a clone of the request hub")
	assert.NoError(t, got.err, "the task must not be canceled with the request")
}

func TestExecuteWithTimeout_KeepsRequestContext(t *testing.T) {
	setupSentryConfig(t)
	reqCtx, reqHub := requestContext(t)

	done := make(chan taskContext, 1)
	ExecuteWithTimeout(reqCtx, time.Second, func(ctx context.Context) error {
		capture(done)(ctx)
		return nil
	})
	got := <-done

	assert.Equal(t, "req-1", got.requestID)
	require.NotNil(t, got.hub)
	assert.NotSame(t, reqHub, got.hub, "the task must not share the hub of the caller")
	assert.NoError(t, got.err)
}

func TestExecuteWithTimeout_Timeout(t *testing.T) {
	setupSentryConfig(t)

	done := make(chan error, 1)
	ExecuteWithTimeout(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		done <- context.Cause(ctx)
		return nil
	})
	assert.True(t, errors.Is(<-done, ErrTaskTimeout))
}
