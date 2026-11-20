package http

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type captureLogger struct {
	fields map[string]any
}

func (l *captureLogger) TraceInfo(_ context.Context, _ string, fields map[string]any) {
	l.fields = fields
}

func (l *captureLogger) TraceError(_ context.Context, _ string, fields map[string]any) {
	l.fields = fields
}

func TestLoggingTransportRedactsSensitiveHeaders(t *testing.T) {
	base := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		// The real request must still carry the original credentials.
		assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))

		h := http.Header{}
		h.Set("Set-Cookie", "sid=abc; HttpOnly")
		h.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})

	logger := &captureLogger{}
	tr := NewLoggingTransport(base, logger, LoggingOptions{
		AllowedReqHeaders:  []string{"Authorization", "X-Api-Key", "Content-Type"},
		AllowedRespHeaders: []string{"Set-Cookie", "Content-Type"},
	})

	ctx := bctx.SetRequestID(context.Background(), "req-1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.invalid/", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("X-Api-Key", "k-123")
	req.Header.Set("Content-Type", "application/json")

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, map[string]any{
		"Authorization":       "Bearer [REDACTED]",
		"X-Api-Key":           "[REDACTED]",
		"Content-Type":        "application/json",
		echo.HeaderXRequestID: "req-1",
	}, logger.fields["request_header"])
	assert.Equal(t, map[string]any{
		"Set-Cookie":   "[REDACTED]",
		"Content-Type": "application/json",
	}, logger.fields["response_header"])
}
