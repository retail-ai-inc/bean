package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"
	berror "github.com/retail-ai-inc/bean/v3/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func panicWithValue(c *echo.Context) error {
	panic("boom")
}

func panicWithNilPointer(c *echo.Context) error {
	var m *struct{ v int }
	return c.String(http.StatusOK, string(rune(m.v))) // nil pointer dereference
}

func runRecover(t *testing.T, h echo.HandlerFunc) error {
	t.Helper()
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	return Recover()(h)(c)
}

// innermostFrame returns the function of the innermost frame Sentry extracts from err.
func innermostFrame(t *testing.T, err error) string {
	t.Helper()
	st := sentry.ExtractStacktrace(err)
	require.NotNil(t, st, "Sentry must be able to extract the stack trace")
	require.NotEmpty(t, st.Frames)
	// Sentry orders frames from the outermost to the innermost call.
	return st.Frames[len(st.Frames)-1].Function
}

func TestRecover_PanicValue(t *testing.T) {
	err := runRecover(t, panicWithValue)

	var pe *berror.PanicError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, "boom", pe.Value)
	assert.Equal(t, "[PANIC RECOVER] boom", err.Error())
	assert.Equal(t, "panicWithValue", innermostFrame(t, err))
}

func TestRecover_RuntimeError(t *testing.T) {
	err := runRecover(t, panicWithNilPointer)

	var re runtime.Error
	assert.ErrorAs(t, err, &re, "the runtime error must be reachable with errors.As")
	assert.Equal(t, "panicWithNilPointer", innermostFrame(t, err), "runtime panic frames must be skipped")

	// `%+v` prints the stack for local logs.
	assert.Contains(t, fmt.Sprintf("%+v", err), "panicWithNilPointer")
}

func TestRecover_NoPanic(t *testing.T) {
	want := errors.New("handler error")
	err := runRecover(t, func(c *echo.Context) error { return want })
	assert.Same(t, want, err)
}

func TestRecover_ErrAbortHandlerIsRepanicked(t *testing.T) {
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		_ = runRecover(t, func(c *echo.Context) error { panic(http.ErrAbortHandler) })
	})
}
