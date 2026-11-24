{{ .Copyright }}
package middlewares

import "github.com/labstack/echo/v5"

func Example(arg string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Pass the request context so the log carries the request ID / trace ID.
			c.Logger().InfoContext(c.Request().Context(), arg)
			return next(c)
		}
	}
}
