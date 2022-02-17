// Copyright The RAI Inc.
// The RAI Authors
package error

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/framework/internals/validator"
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
		// Send error event to sentry.
		sentry.CaptureException(he)
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

	// Send error event to sentry.
	sentry.CaptureException(he)

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
	// Send error event to sentry.
	sentry.CaptureException(err)

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
