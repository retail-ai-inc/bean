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
	"net/http"

	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/validator"
	"github.com/spf13/viper"
)

type errorResp struct {
	ErrorCode ErrorCode   `json:"errorCode"`
	ErrorMsg  interface{} `json:"errorMsg"`
}

type ErrorHandlerFunc func(err error, c echo.Context) (bool, error)

func ValidationErrorHanderFunc(e error, c echo.Context) (bool, error) {
	he, ok := e.(*validator.ValidationError)
	if !ok {
		return false, nil
	}
	err := c.JSON(http.StatusBadRequest, errorResp{
		ErrorCode: API_DATA_VALIDATION_FAILED,
		ErrorMsg:  he.ErrCollection(),
	})

	return ok, err
}

func APIErrorHanderFunc(e error, c echo.Context) (bool, error) {
	he, ok := e.(*APIError)
	if !ok {
		return false, nil
	}

	if he.HTTPStatusCode >= 404 {
		// Send error event to sentry if configured.
		if viper.GetBool("sentry.on") {
			if hub := sentryecho.GetHubFromContext(c); hub != nil {
				hub.CaptureException(he)
			}
		}

		c.Logger().Error(he)
	}

	err := c.JSON(he.HTTPStatusCode, errorResp{
		ErrorCode: he.GlobalErrCode,
		ErrorMsg:  he.Error(),
	})

	return ok, err
}

func EchoHTTPErrorHanderFunc(e error, c echo.Context) (bool, error) {
	he, ok := e.(*echo.HTTPError)
	if !ok {
		return false, nil
	}

	// Send error event to sentry if configured.
	if viper.GetBool("sentry.on") {
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureException(he)
		}
	}

	c.Logger().Error(he)

	// Return different response base on some defined error.
	var err error
	switch he.Code {
	case http.StatusNotFound:
		err = c.JSON(he.Code, errorResp{
			ErrorCode: RESOURCE_NOT_FOUND,
			ErrorMsg:  he.Message,
		})
	case http.StatusMethodNotAllowed:
		err = c.JSON(he.Code, errorResp{
			ErrorCode: METHOD_NOT_ALLOWED,
			ErrorMsg:  he.Message,
		})
	default:
		err = c.JSON(he.Code, errorResp{
			ErrorCode: UNKNOWN_ERROR_CODE,
			ErrorMsg:  he.Message,
		})
	}

	return ok, err
}

func DefaultErrorHanderFunc(err error, c echo.Context) (bool, error) {
	// Send error event to sentry if configured.
	if viper.GetBool("sentry.on") {
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		}
	}

	c.Logger().Error(err)

	// Get Content-Type parameter from request header to identify the request content type. If the request is for
	// html then we should display the error in html.
	if c.Request().Header.Get("Content-Type") == "text/html" {
		err := c.HTML(http.StatusInternalServerError, "<strong>Internal server error.</strong>")
		return true, err
	}

	// All other uncaught errors.
	// Sentry already captured the panic and send notification in sentry-recover middleware.
	err = c.JSON(http.StatusInternalServerError, errorResp{
		ErrorCode: INTERNAL_SERVER_ERROR,
		ErrorMsg:  err.Error(),
	})

	return true, err
}

func OnTimeoutRouteErrorHandler(err error, c echo.Context) {
	// Send error event to sentry if configured.
	if viper.GetBool("sentry.on") {
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		}
	}

	c.Logger().Error(err)
}
