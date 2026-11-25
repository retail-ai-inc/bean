package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tasks of a pool get a clone of the caller's hub (with the request in its scope), not the caller's hub.
func TestNewPool_TasksUseAClonedHub(t *testing.T) {
	prev := config.Bean
	config.Bean = &config.Config{Sentry: config.Sentry{On: true}}
	t.Cleanup(func() { config.Bean = prev })

	parentHub := sentry.CurrentHub().Clone()
	ctx := sentry.SetHubOnContext(context.Background(), parentHub)

	p := NewPool(ctx, WithRequest(httptest.NewRequest(http.MethodGet, "/orders", nil)))
	var taskHub *sentry.Hub
	p.Go(func(ctx context.Context) error {
		taskHub = sentry.GetHubFromContext(ctx)
		return nil
	})
	require.NoError(t, p.Wait())

	require.NotNil(t, taskHub)
	assert.NotSame(t, parentHub, taskHub)
}
