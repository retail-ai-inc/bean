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
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/internal/validator"
	"github.com/spf13/viper"
)

// ErrorCode represents the error code of the API.
type ErrorResp struct {
	ErrorCode ErrorCode   `json:"errorCode"`
	ErrorMsg  interface{} `json:"errorMsg"`
	Errors    interface{} `json:"errors"`
}

type ErrorHandlerFunc func(err error, c echo.Context) (bool, error)

func ValidationErrorHandlerFunc(e error, c echo.Context) (bool, error) {
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
func APIErrorHandlerFunc(e error, c echo.Context) (bool, error) {
	var ae *APIError
	if !errors.As(e, &ae) {
		return false, nil
	}

	if ae.HTTPStatusCode >= 404 {
		c.Logger().Error(ae.Error())

		if ae.HTTPStatusCode > 404 {
			// Send error event to sentry if configured.
			if viper.GetBool("sentry.on") {
				if hub := sentryecho.GetHubFromContext(c); hub != nil {
					hub.CaptureException(ae)
				}
			}
		}
	} else {
		c.Logger().Error(ae.Error())
	}

	err := c.JSON(ae.HTTPStatusCode, ErrorResp{
		ErrorCode: ae.GlobalErrCode,
		ErrorMsg:  ae.Error(),
	})

	return true, err
}

func HTTPErrorHandlerFunc(e error, c echo.Context) (bool, error) {
	var he *echo.HTTPError
	if !errors.As(e, &he) {
		return false, nil
	}

	c.Logger().Error(he.Error())

	// Return different response based on some defined error.
	var err error
	switch he.Code {
	case http.StatusNotFound:
		if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
			// Get from env.json file.
			html404File := viper.GetString("http.errorMessage.e404.html.file")
			if html404File != "" {
				err = c.Render(he.Code, html404File, echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			} else {
				err = c.Render(he.Code, "errors/html/404", echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			}
		} else {
			// Get from env.json file.
			e404 := viper.GetStringMap("http.errorMessage.e404")
			if val, ok := e404["json"]; ok {
				err = c.JSON(he.Code, converter(val))
			} else {
				err = c.JSON(he.Code, ErrorResp{ErrorCode: RESOURCE_NOT_FOUND, ErrorMsg: he.Message})
			}
		}

	case http.StatusMethodNotAllowed:
		if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
			err = c.Render(he.Code, "errors/html/405", echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
		} else {
			// Get from env.json file.
			e405 := viper.GetStringMap("http.errorMessage.e405")
			if val, ok := e405["json"]; ok {
				err = c.JSON(he.Code, converter(val))
			} else {
				err = c.JSON(he.Code, ErrorResp{ErrorCode: METHOD_NOT_ALLOWED, ErrorMsg: he.Message})
			}
		}

	case http.StatusInternalServerError:
		// Send error event to sentry if configured.
		if viper.GetBool("sentry.on") {
			if hub := sentryecho.GetHubFromContext(c); hub != nil {
				hub.CaptureException(he)
			}
		}

		if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
			// Get from env.json file.
			html500File := viper.GetString("http.errorMessage.e500.html.file")
			if html500File != "" {
				err = c.Render(he.Code, html500File, echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			} else {
				err = c.Render(he.Code, "errors/html/500", echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			}
		} else {
			// Get from env.json file.
			def := viper.GetStringMap("http.errorMessage.e500")
			if val, ok := def["json"]; ok {
				err = c.JSON(he.Code, converter(val))
			} else {
				err = c.JSON(he.Code, ErrorResp{ErrorCode: INTERNAL_SERVER_ERROR, ErrorMsg: he.Message})
			}
		}

	case http.StatusGatewayTimeout:
		if viper.GetBool("sentry.on") {
			if hub := sentryecho.GetHubFromContext(c); hub != nil {
				hub.CaptureException(he.Internal)
			}
		}

		if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
			// Get from env.json file.
			html504File := viper.GetString("http.errorMessage.e504.html.file")
			if html504File != "" {
				err = c.Render(he.Code, html504File, echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			} else {
				err = c.Render(he.Code, "errors/html/504", echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			}
		} else {
			// Get from env.json file.
			e504 := viper.GetStringMap("http.errorMessage.e504")
			if val, ok := e504["json"]; ok {
				err = c.JSON(he.Code, converter(val))
			} else {
				err = c.JSON(he.Code, ErrorResp{ErrorCode: TIMEOUT, ErrorMsg: he.Message})
			}
		}

	default:
		// Send error event to sentry if configured.
		if viper.GetBool("sentry.on") {
			if hub := sentryecho.GetHubFromContext(c); hub != nil {
				hub.CaptureException(he)
			}
		}

		if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
			// Get from env.json file.
			html500File := viper.GetString("http.errorMessage.default.html.file")
			if html500File != "" {
				err = c.Render(he.Code, html500File, echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			} else {
				err = c.Render(he.Code, "errors/html/500", echo.Map{"stacktrace": fmt.Sprintf("%+v", e)})
			}
		} else {
			// Get from env.json file.
			def := viper.GetStringMap("http.errorMessage.default")
			if val, ok := def["json"]; ok {
				err = c.JSON(he.Code, converter(val))
			} else {
				err = c.JSON(he.Code, ErrorResp{ErrorCode: INTERNAL_SERVER_ERROR, ErrorMsg: he.Message})
			}
		}
	}

	return true, err
}

// If any other error handler doesn't catch the error then finally `DefaultErrorHandlerFunc` will
// cactch the error and treat all those errors as `http.StatusInternalServerError`.
func DefaultErrorHandlerFunc(err error, c echo.Context) (bool, error) {
	// Send error event to sentry if configured.
	if viper.GetBool("sentry.on") {
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		}
	}

	if err != nil {
		c.Logger().Error(err.Error())
	} else {
		c.Logger().Error("unknown error")
	}

	// Get Content-Type parameter from request header to identify the request content type. If the request is for
	// html then we should display the error in html.
	if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
		// Get from env.json file.
		html500File := viper.GetString("http.errorMessage.default.html.file")
		if html500File != "" {
			return true, c.Render(http.StatusInternalServerError, html500File, echo.Map{
				"stacktrace": fmt.Sprintf("%+v", err),
			})
		} else {
			return true, c.Render(http.StatusInternalServerError, "errors/html/500", echo.Map{
				"stacktrace": fmt.Sprintf("%+v", err),
			})
		}
	}

	// If the Content-Type is `application/json` then return JSON response.
	// Get from env.json file.
	def := viper.GetStringMap("http.errorMessage.default")
	if val, ok := def["json"]; ok {
		err = c.JSON(http.StatusInternalServerError, converter(val))
	} else {
		err = c.JSON(http.StatusInternalServerError, ErrorResp{
			ErrorCode: INTERNAL_SERVER_ERROR,
			ErrorMsg:  err.Error(),
		})
	}

	return true, err
}

func converter(data interface{}) interface{} {
	slice, ok := data.([]interface{})
	if !ok {
		return data
	}

	var message = make(map[string]interface{})
	for _, v := range slice {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		message[m["key"].(string)] = m["value"]
	}
	return message
}
