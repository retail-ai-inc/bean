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
	"fmt"
	"net/http"
	"strings"

	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v5"
	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/retail-ai-inc/bean/v3/internal/validator"
)

// ErrorCode represents the error code of the API.
type ErrorResp struct {
	ErrorCode ErrorCode   `json:"errorCode"`
	ErrorMsg  interface{} `json:"errorMsg"`
	Errors    interface{} `json:"errors"`
}

type ErrorHandlerFunc func(err error, c *echo.Context) (bool, error)

func ValidationErrorHandlerFunc(e error, c *echo.Context) (bool, error) {
	var ve *validator.ValidationError
	if !errors.As(e, &ve) {
		return false, nil
	}

	c.Logger().Error(ve.Error())

	err := c.JSON(http.StatusBadRequest, ErrorResp{
		ErrorCode: API_DATA_VALIDATION_FAILED,
		Errors:    ve.ErrCollection(),
	})

	return true, err
}

// Default JSON API error handler. The response pattern is like below:
// `{"errorCode": "1000001", "errorMsg": "some message"}`. You can override this error handler from `start.go`
func APIErrorHandlerFunc(e error, c *echo.Context) (bool, error) {
	var ae *APIError
	if !errors.As(e, &ae) {
		return false, nil
	}

	// Log HTTP errors (status >= 404) to Sentry if enabled, else log stack trace locally.
	// 404 threshold prevents bloating Sentry with minor errors.
	if ae.HTTPStatusCode >= 404 && sentryOn() {
		// Sends stack to Sentry without logging to
		// avoid duplication, as error is already handled explicitly.
		captureException(c, ae)
	} else {
		// logs the stack trace for all errors when sentry is disabled.
		c.Logger().Error(fmt.Sprintf("%+v", ae))
	}

	err := c.JSON(ae.HTTPStatusCode, ErrorResp{
		ErrorCode: ae.GlobalErrCode,
		ErrorMsg:  ae.Error(),
	})

	return true, err
}

// HTTPErrorHandlerFunc handles errors carrying an HTTP status: `*echo.HTTPError` and the echo predefined
// errors such as `echo.ErrNotFound` (in echo v5 these only implement `echo.HTTPStatusCoder`).
// The response is configured per status by `http.errorMessage.*` in env.json.
func HTTPErrorHandlerFunc(e error, c *echo.Context) (bool, error) {
	// `APIError` and `ValidationError` also carry an HTTP status; they have their own handlers.
	var ae *APIError
	var ve *validator.ValidationError
	if errors.As(e, &ae) || errors.As(e, &ve) {
		return false, nil
	}

	var sc echo.HTTPStatusCoder
	if !errors.As(e, &sc) {
		return false, nil
	}

	he := newHTTPErrorInfo(sc)

	// Requests to unknown routes / methods are common (scanners, typos), do not log them as errors.
	if !IsRouterError(c) {
		c.Logger().Error(e.Error())
	}

	page := errorPageFor(he.Code)
	if page.capture && sentryOn() {
		if page.captureInternal {
			captureException(c, he.Internal)
		} else {
			captureException(c, he.err)
		}
	}

	return true, renderErrorPage(c, he.Code, page, he.Message, e)
}

// If any other error handler doesn't catch the error then finally `DefaultErrorHandlerFunc` will
// cactch the error and treat all those errors as `http.StatusInternalServerError`.
func DefaultErrorHandlerFunc(err error, c *echo.Context) (bool, error) {

	if err == nil {
		return false, nil
	}

	// 404 / 405 produced by the router are always rendered as 404 / 405, even when
	// `HTTPErrorHandlerFunc` is not registered.
	if IsRouterError(c) {
		return HTTPErrorHandlerFunc(err, c)
	}

	// Send error event to sentry if configured.
	if sentryOn() {
		c.Logger().Error(err.Error())
		captureException(c, err)
	} else {
		c.Logger().Error(fmt.Sprintf("%+v", err))
	}

	return true, renderErrorPage(c, http.StatusInternalServerError, defaultErrorPage(), err.Error(), err)
}

// errorPage describes how an HTTP error status is rendered and reported.
type errorPage struct {
	config.ErrorPage           // customization from `http.errorMessage.*`
	defaultHTML      string    // template used when `Html.File` is not configured
	errCode          ErrorCode // `errorCode` of the default JSON body
	capture          bool      // report the error to Sentry
	captureInternal  bool      // report the wrapped error instead of the HTTP error
}

func errorPageFor(code int) errorPage {
	msgs := beanConfig().HTTP.ErrorMessage
	switch code {
	case http.StatusNotFound:
		return errorPage{ErrorPage: msgs.E404, defaultHTML: "errors/html/404", errCode: RESOURCE_NOT_FOUND}
	case http.StatusMethodNotAllowed:
		return errorPage{ErrorPage: msgs.E405, defaultHTML: "errors/html/405", errCode: METHOD_NOT_ALLOWED}
	case http.StatusInternalServerError:
		return errorPage{ErrorPage: msgs.E500, defaultHTML: "errors/html/500", errCode: INTERNAL_SERVER_ERROR, capture: true}
	case http.StatusGatewayTimeout:
		return errorPage{ErrorPage: msgs.E504, defaultHTML: "errors/html/504", errCode: TIMEOUT, capture: true, captureInternal: true}
	default:
		return defaultErrorPage()
	}
}

func defaultErrorPage() errorPage {
	return errorPage{
		ErrorPage:   beanConfig().HTTP.ErrorMessage.Default,
		defaultHTML: "errors/html/500",
		errCode:     INTERNAL_SERVER_ERROR,
		capture:     true,
	}
}

// renderErrorPage renders the HTML page, or the JSON body when the request is a JSON request.
func renderErrorPage(c *echo.Context, code int, page errorPage, errMsg string, stack error) error {
	// Get Content-Type parameter from request header to identify the request content type. If the request is for
	// html then we should display the error in html.
	if !strings.Contains(c.Request().Header.Get(echo.HeaderContentType), echo.MIMEApplicationJSON) {
		file := page.Html.File
		if file == "" {
			file = page.defaultHTML
		}
		return c.Render(code, file, map[string]any{"stacktrace": fmt.Sprintf("%+v", stack)})
	}

	if body := page.JSONBody(); body != nil {
		return c.JSON(code, body)
	}
	return c.JSON(code, ErrorResp{ErrorCode: page.errCode, ErrorMsg: errMsg})
}

func beanConfig() *config.Config {
	if config.Bean == nil {
		return &config.Config{}
	}
	return config.Bean
}

func sentryOn() bool {
	return beanConfig().Sentry.On
}

func captureException(c *echo.Context, err error) {
	if err == nil {
		return
	}
	if hub := sentryecho.GetHubFromContext(c); hub != nil {
		hub.CaptureException(err)
	}
}

// httpErrorInfo holds the information extracted from an echo v5 HTTP error
// (`*echo.HTTPError` or one of the predefined `echo.Err*` sentinels).
type httpErrorInfo struct {
	Code     int
	Message  string
	Internal error // the wrapped error, falls back to the HTTP error itself.
	err      error // the HTTP error itself.
}

func newHTTPErrorInfo(sc echo.HTTPStatusCoder) httpErrorInfo {
	code := sc.StatusCode()
	info := httpErrorInfo{Code: code, Message: http.StatusText(code)}

	if err, ok := sc.(error); ok {
		info.err = err
		info.Internal = err
	}

	if he, ok := sc.(*echo.HTTPError); ok {
		if he.Message != "" {
			info.Message = he.Message
		}
		if internal := he.Unwrap(); internal != nil {
			info.Internal = internal
		}
	}

	return info
}

// IsRouterError reports whether the request did not match any route (404) or matched a path but not the
// method (405). In these cases the error was produced by the echo router, not by a handler.
func IsRouterError(c *echo.Context) bool {
	switch c.RouteInfo().Name {
	case echo.NotFoundRouteName, echo.MethodNotAllowedRouteName:
		return true
	}
	return false
}
