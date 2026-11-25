package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLogger struct {
	mu     sync.Mutex
	fields []map[string]any
}

func (l *fakeLogger) TraceInfo(_ context.Context, _ string, fields map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fields = append(l.fields, fields)
}

func (l *fakeLogger) TraceError(ctx context.Context, level string, fields map[string]any) {
	l.TraceInfo(ctx, level, fields)
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error { b.closed = true; return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLoggingTransport_ResponseBodyCloseReachesOriginal(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader("0123456789")}
	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body}, nil
	})
	logger := &fakeLogger{}
	rt := NewLoggingTransport(base, logger, LoggingOptions{DumpBody: true, BodyLimit: 4})

	req, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "0123456789", string(got))
	require.NoError(t, resp.Body.Close())
	assert.True(t, body.closed, "closing the response body must close the original body")
	assert.Equal(t, "0123", logger.fields[0]["response_body"])
}

func TestLoggingTransport_SwitchingProtocolsBodyNotWrapped(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader("")}
	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusSwitchingProtocols, Header: http.Header{}, Body: body}, nil
	})
	rt := NewLoggingTransport(base, &fakeLogger{}, LoggingOptions{DumpBody: true})

	req, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Same(t, body, resp.Body)
}

func TestLoggingTransport_RequestBody(t *testing.T) {
	const payload = "request-payload-longer-than-limit"

	for name, newBody := range map[string]func() io.Reader{
		// http.NewRequest sets GetBody for a strings.Reader: the prefix is read from a copy.
		"with GetBody": func() io.Reader { return strings.NewReader(payload) },
		// No GetBody: the prefix is read from the body and replayed.
		"without GetBody": func() io.Reader { return io.MultiReader(strings.NewReader(payload)) },
	} {
		t.Run(name, func(t *testing.T) {
			var sent string
			base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				b, err := io.ReadAll(r.Body)
				sent = string(b)
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: http.NoBody}, err
			})
			logger := &fakeLogger{}
			rt := NewLoggingTransport(base, logger, LoggingOptions{DumpBody: true, BodyLimit: 7})

			req, err := http.NewRequest(http.MethodPost, "http://example.test/", newBody())
			require.NoError(t, err)
			_, err = rt.RoundTrip(req)
			require.NoError(t, err)

			assert.Equal(t, payload, sent, "the whole body must be sent")
			assert.Equal(t, payload[:7], logger.fields[0]["request_body"])
		})
	}
}

type failingBody struct {
	closed bool
}

func (b *failingBody) Read([]byte) (int, error) { return 0, errors.New("disk error") }
func (b *failingBody) Close() error             { b.closed = true; return nil }

// A request body read error must fail the request instead of sending a truncated body.
func TestLoggingTransport_RequestBodyReadError(t *testing.T) {
	called := false
	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: http.NoBody}, nil
	})
	rt := NewLoggingTransport(base, &fakeLogger{}, LoggingOptions{DumpBody: true})

	body := &failingBody{}
	req, err := http.NewRequest(http.MethodPost, "http://example.test/", body)
	require.NoError(t, err)
	_, err = rt.RoundTrip(req)
	assert.EqualError(t, err, "disk error")
	assert.False(t, called)
	assert.True(t, body.closed)
}
