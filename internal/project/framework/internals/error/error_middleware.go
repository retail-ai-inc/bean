/**#bean*/ /*#bean.replace({{ .Copyright }})**/

package error

import (
	/**#bean*/
	"demo/framework/internals/sentry"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/sentry")**/
	/**#bean*/
	"demo/framework/internals/validator"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/validator")**/
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// return value: bool is true when HandlerMiddleware match the error,otherwise false
// return value: error will be sent to sentry if not nil

type ErrorHandlerMiddleware func(err error, c echo.Context) (bool, error)

func ErrorHandlerChain(middlewares ...ErrorHandlerMiddleware) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {

		if c.Response().Committed {
			return
		}

		for _, middleware := range middlewares {
			catched, e := middleware(err, c)
			if e != nil {
				sentry.PushData(c, e, nil, true)
			}
			if catched {
				break
			}
		}
	}
}

func ValidationErrorHanderMiddleware(e error, c echo.Context) (bool, error) {
	he, ok := e.(*validator.ValidationError)
	if !ok {
		return false, nil
	}
	err := c.JSON(http.StatusUnprocessableEntity, errorResp{
		ErrorCode: API_DATA_VALIDATION_FAILED,
		Errors:    he.ErrCollection(),
		ErrorMsg:  nil,
	})

	return ok, err
}

func APIErrorHanderMiddleware(e error, c echo.Context) (bool, error) {
	he, ok := e.(*APIError)
	if !ok {
		return false, nil
	}

	if he.HTTPStatusCode >= 404 {
		sentry.PushData(c, he, nil, true)
	}

	err := c.JSON(he.HTTPStatusCode, errorResp{
		ErrorCode: he.GlobalErrCode,
		Errors:    nil,
		ErrorMsg:  he.Error(),
	})

	return ok, err
}

func HTTPErrorHanderMiddleware(e error, c echo.Context) (bool, error) {
	he, ok := e.(*echo.HTTPError)
	if !ok {
		return false, nil
	}

	// Just in case to capture this unused type error.
	var err error
	switch he.Code {
	case http.StatusNotFound:
		err = c.JSON(he.Code, errorResp{
			ErrorCode: RESOURCE_NOT_FOUND,
			Errors:    nil,
			ErrorMsg:  he.Message,
		})
	case http.StatusMethodNotAllowed:
		err = c.JSON(he.Code, errorResp{
			ErrorCode: METHOD_NOT_ALLOWED,
			Errors:    nil,
			ErrorMsg:  he.Message,
		})
	default:
		err = c.JSON(he.Code, errorResp{
			ErrorCode: UNKNOWN_ERROR_CODE,
			Errors:    nil,
			ErrorMsg:  he.Message,
		})
	}

	return ok, err
}

func DefaultErrorHanderMiddleware(_ error, c echo.Context) (bool, error) {
	// Get Content-Type parameter from request header to identify the request content type. If the request is for
	// html then we should display the error in html.
	contentType := c.Request().Header.Get("Content-Type")

	if strings.ToLower(contentType) == "text/html" {
		err := c.HTML(http.StatusInternalServerError, "<strong>Internal server error.</strong>")
		return true, err
	}

	// All other panic errors.
	// Sentry already captured the panic and send notification in sentry-recover middleware.
	err := c.JSON(http.StatusInternalServerError, errorResp{
		ErrorCode: INTERNAL_SERVER_ERROR,
		Errors:    nil,
		ErrorMsg:  nil, // TODO: Put some generic message.
	})

	return true, err
}
