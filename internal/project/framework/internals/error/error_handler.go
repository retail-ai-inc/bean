/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package error

import (
	"net/http"
	"strings"

	/**#bean*/
	"demo/framework/internals/sentry"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/sentry")**/
	/**#bean*/
	"demo/framework/internals/validator"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/validator")**/

	"github.com/labstack/echo/v4"
)

type errorResp struct {
	ErrorCode ErrorCode           `json:"errorCode"`
	Errors    []map[string]string `json:"errors"`
	ErrorMsg  interface{}         `json:"errorMsg"`
}

// HTTPErrorHandler is a middleware which handles all the error.
func HTTPErrorHandler(err error, c echo.Context) {

	if c.Response().Committed {
		return
	}

	switch he := err.(type) {

	case *validator.ValidationError:

		err := c.JSON(http.StatusUnprocessableEntity, errorResp{
			ErrorCode: API_DATA_VALIDATION_FAILED,
			Errors:    he.ErrCollection(),
			ErrorMsg:  nil,
		})

		if err != nil {
			sentry.PushData(c, err, nil, true)
		}

		return

	case *APIError:

		if he.HTTPStatusCode >= 404 {
			sentry.PushData(c, he, nil, true)
		}

		err := c.JSON(he.HTTPStatusCode, errorResp{
			ErrorCode: he.GlobalErrCode,
			Errors:    nil,
			ErrorMsg:  he.Error(),
		})

		if err != nil {
			sentry.PushData(c, err, nil, true)
		}

		return

	case *echo.HTTPError:

		// Just in case to capture this unused type error.
		err := c.JSON(he.Code, errorResp{
			ErrorCode: UNKNOWN_ERROR_CODE,
			Errors:    nil,
			ErrorMsg:  he.Message,
		})

		if err != nil {
			sentry.PushData(c, err, nil, true)
		}

		return

	default:

		// Get Content-Type parameter from request header to identify the request content type. If the request is for
		// html then we should display the error in html.
		contentType := c.Request().Header.Get("Content-Type")

		if strings.ToLower(contentType) == "text/html" {
			c.HTML(http.StatusInternalServerError, "<strong>Internal server error.</strong>")
			return
		}

		// All other panic errors.
		// Sentry already captured the panic and send notification in sentry-recover middleware.
		err := c.JSON(http.StatusInternalServerError, errorResp{
			ErrorCode: INTERNAL_SERVER_ERROR,
			Errors:    nil,
			ErrorMsg:  nil, // TODO: Put some generic message.
		})

		if err != nil {
			sentry.PushData(c, err, nil, true)
		}

		return
	}
}
