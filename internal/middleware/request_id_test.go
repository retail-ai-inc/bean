package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDHandlerSetsSentryTag(t *testing.T) {

	tests := []struct {
		name      string
		ctxReqID  string
		useEchoKV bool
		wantTag   string
		wantID    string
	}{
		{name: "hub from echo context", useEchoKV: true, wantTag: "generated", wantID: "generated"},
		{name: "existing context request id wins", ctxReqID: "existing", useEchoKV: true, wantTag: "existing", wantID: "existing"},
		{name: "hub only in request context is not tagged", wantID: "generated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := sentry.NewHub(nil, sentry.NewScope())
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			ctx := req.Context()
			if tt.ctxReqID != "" {
				ctx = bctx.SetRequestID(ctx, tt.ctxReqID)
			}
			if !tt.useEchoKV {
				ctx = sentry.SetHubOnContext(ctx, hub)
			}
			req = req.WithContext(ctx)

			c := echo.New().NewContext(req, httptest.NewRecorder())
			if tt.useEchoKV {
				c.Set("sentry", hub)
			}

			RequestIDHandler(c, "generated")

			event := hub.Scope().ApplyToEvent(sentry.NewEvent(), nil, nil)
			require.NotNil(t, event)
			if tt.wantTag == "" {
				assert.NotContains(t, event.Tags, "request_id")
			} else {
				assert.Equal(t, tt.wantTag, event.Tags["request_id"])
			}

			gotID, ok := bctx.GetRequestID(c.Request().Context())
			assert.True(t, ok)
			assert.Equal(t, tt.wantID, gotID)
		})
	}
}

func TestRequestIDHandlerWithoutSentryHub(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := echo.New().NewContext(req, httptest.NewRecorder())

	assert.NotPanics(t, func() { RequestIDHandler(c, "generated") })
	assert.Equal(t, "generated", c.Request().Header.Get(echo.HeaderXRequestID))
	assert.Equal(t, "generated", c.Get(echo.HeaderXRequestID))
}
