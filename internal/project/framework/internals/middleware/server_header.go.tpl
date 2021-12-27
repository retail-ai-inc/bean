{{ .Copyright }}
package middleware

import (
	"github.com/labstack/echo/v4"
)

// ServerHeader middleware adds a `Server` header to the response.
func ServerHeader(name, version string) echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			c.Response().Header().Set(echo.HeaderServer, name+"/"+version)
			return next(c)
		}
	}
}
