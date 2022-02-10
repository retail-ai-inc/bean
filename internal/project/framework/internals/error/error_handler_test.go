/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package error

import (
	/**#bean*/
	ivalidator "demo/framework/internals/validator"
	/*#bean.replace(ivalidator "{{ .PkgPath }}/framework/internals/validator")**/

	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type dummyWriter struct{ io.Writer }

func (dummyWriter) Header() http.Header        { return http.Header{} }
func (dummyWriter) WriteHeader(statusCode int) {}

type fakeError struct{ Message string }

func (f *fakeError) Error() string { return f.Message }

func TestValidationErrorHanderFunc(t *testing.T) {
	e := echo.New()
	c := e.AcquireContext()
	c.SetRequest(httptest.NewRequest("", "/", nil))
	c.SetResponse(echo.NewResponse(dummyWriter{io.Discard}, e))

	fakeErr := &fakeError{"fake"}
	got, err := ValidationErrorHanderFunc(fakeErr, c)
	assert.NoError(t, err)
	assert.Equal(t, false, got)

	validateErr := &ivalidator.ValidationError{Err: validator.ValidationErrors{}}
	got, err = ValidationErrorHanderFunc(validateErr, c)
	assert.NoError(t, err)
	assert.Equal(t, true, got)

	e.ReleaseContext(c)
}

func TestAPIErrorHanderFunc(t *testing.T) {
	e := echo.New()
	c := e.AcquireContext()
	c.SetRequest(httptest.NewRequest("", "/", nil))
	c.SetResponse(echo.NewResponse(dummyWriter{io.Discard}, e))

	fakeErr := &fakeError{"fake"}
	got, err := APIErrorHanderFunc(fakeErr, c)
	assert.NoError(t, err)
	assert.Equal(t, false, got)

	apiErr := NewAPIError(http.StatusInternalServerError, INTERNAL_SERVER_ERROR, errors.New("internal"))
	got, err = APIErrorHanderFunc(apiErr, c)
	assert.NoError(t, err)
	assert.Equal(t, true, got)

	e.ReleaseContext(c)
}

func TestEchoHTTPErrorHanderFunc(t *testing.T) {
	e := echo.New()
	c := e.AcquireContext()
	c.SetRequest(httptest.NewRequest("", "/", nil))
	c.SetResponse(echo.NewResponse(dummyWriter{io.Discard}, e))

	fakeErr := &fakeError{"fake"}
	got, err := EchoHTTPErrorHanderFunc(fakeErr, c)
	assert.NoError(t, err)
	assert.Equal(t, false, got)

	echoHTTPErr := echo.NewHTTPError(http.StatusInternalServerError, "internal")
	got, err = EchoHTTPErrorHanderFunc(echoHTTPErr, c)
	assert.NoError(t, err)
	assert.Equal(t, true, got)

	e.ReleaseContext(c)
}

func TestDefaultErrorHanderFunc(t *testing.T) {
	e := echo.New()
	c := e.AcquireContext()
	c.SetRequest(httptest.NewRequest("", "/", nil))
	c.SetResponse(echo.NewResponse(dummyWriter{io.Discard}, e))

	fakeErr := &fakeError{"fake"}
	got, err := DefaultErrorHanderFunc(fakeErr, c)
	assert.NoError(t, err)
	assert.Equal(t, true, got)

	anyErr := echo.NewHTTPError(http.StatusInternalServerError, "internal")
	got, err = DefaultErrorHanderFunc(anyErr, c)
	assert.NoError(t, err)
	assert.Equal(t, true, got)

	e.ReleaseContext(c)
}
