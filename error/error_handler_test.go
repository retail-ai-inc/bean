// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package error

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	validatorV10 "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/validator"
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

	validateErr := &validator.ValidationError{Err: validatorV10.ValidationErrors{}}
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
