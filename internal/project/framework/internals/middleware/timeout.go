/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package middleware

import (
	"context"
	"time"

	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
)

// RequestTimeout attach a timeout context to the request.
func RequestTimeout(timeout time.Duration, sentryOn bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Start a sentry span for tracing.
			if sentryOn {
				span := sentry.StartSpan(c.Request().Context(), "middleware")
				span.Description = helpers.CurrFuncName()
				defer span.Finish()
			}
			timeoutCtx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			c.SetRequest(c.Request().WithContext(timeoutCtx))
			defer cancel()
			return next(c)
		}
	}
}
