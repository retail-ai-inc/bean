{{ .Copyright }}
package middlewares

import "github.com/labstack/echo/v4"

func Example(arg string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Logger().Info(arg)
			return next(c)
		}
	}
}
