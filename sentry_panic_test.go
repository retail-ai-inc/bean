package bean

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"
	berror "github.com/retail-ai-inc/bean/v3/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func panickingTestHandler(c *echo.Context) error {
	panic("boom")
}

// A panic must be reported to Sentry once, with the stack trace of the panic site.
func TestNewEcho_PanicIsReportedToSentryWithPanicStack(t *testing.T) {
	transport := &sentry.MockTransport{}
	origClient := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(origClient) })

	conf := validTestConfig()
	conf.Sentry.On = true
	conf.Sentry.ClientOptions = &sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: transport,
	}
	useConfig(t, conf)

	e, closeEcho, err := NewEcho()
	require.NoError(t, err)
	t.Cleanup(func() { _ = closeEcho() })

	b := &Bean{Echo: e}
	b.UseErrorHandlerFuncs(berror.DefaultErrorHandlerFunc)
	e.HTTPErrorHandler = b.DefaultHTTPErrorHandler()
	e.GET("/panic", panickingTestHandler)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	events := transport.Events()
	require.Len(t, events, 1, "the panic must be reported exactly once")
	require.NotEmpty(t, events[0].Exception)

	var frames []sentry.Frame
	for _, ex := range events[0].Exception {
		if ex.Stacktrace != nil && len(ex.Stacktrace.Frames) > 0 {
			frames = ex.Stacktrace.Frames
		}
	}
	require.NotEmpty(t, frames, "the exception must carry a stack trace")
	assert.Equal(t, "panickingTestHandler", frames[len(frames)-1].Function,
		"the innermost frame must be the panic site, not where the error was reported")
}
