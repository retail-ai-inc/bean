/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package middleware

import (
	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
)

// ServerHeader middleware adds a `Server` header to the response.
func ServerHeader(name, version string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			// Start a sentry span for tracing.
			span := sentry.StartSpan(c.Request().Context(), "middleware")
			span.Description = helpers.CurrFuncName()
			defer span.Finish()
			c.Response().Header().Set(echo.HeaderServer, name+"/"+version)
			return next(c)
		}
	}
}
