// Copyright The RAI Inc.
// The RAI Authors
package bean

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestBean_UseErrorHandlerFuncs(t *testing.T) {
	b := &Bean{}
	assert.Empty(t, b.errorHandlerFuncs)

	b.UseErrorHandlerFuncs(func(err error, c echo.Context) (bool, error) {
		return true, nil
	})
	assert.Equal(t, 1, len(b.errorHandlerFuncs))
}

func TestDefaultHTTPErrorHandler(t *testing.T) {
	b := &Bean{}
	b.Echo = echo.New()

	b.UseErrorHandlerFuncs(
		func(err error, c echo.Context) (bool, error) {
			he, ok := err.(*fakeError)
			if !ok {
				return false, nil
			}
			err = c.JSON(http.StatusBadRequest, map[string]interface{}{
				"errorCode": "fake code",
				"errors":    he.Error(),
			})
			return ok, err
		},
		func(_ error, c echo.Context) (bool, error) {
			err := c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"errorCode": "default code",
				"errors":    "default catched!",
			})
			return true, err
		},
	)

	b.Echo.HTTPErrorHandler = b.DefaultHTTPErrorHandler()

	b.Echo.Any("/fake", func(c echo.Context) error {
		return newFakeError("fake error")
	})
	b.Echo.Any("/default", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "default error")
	})

	// With Debug=true plain response contains error message
	code, body := request(http.MethodGet, "/fake", b.Echo)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, `{"errorCode":"fake code","errors":"fake error"}`+"\n", body)
	// and special handling for HTTPError
	code, body = request(http.MethodGet, "/default", b.Echo)
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, `{"errorCode":"default code","errors":"default catched!"}`+"\n", body)
}

func request(method, path string, e *echo.Echo) (int, string) {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

type fakeError struct {
	Message string
}

func (f *fakeError) Error() string {
	return f.Message
}

func newFakeError(msg string) error {
	return &fakeError{
		Message: msg,
	}
}
