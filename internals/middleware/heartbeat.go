/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

/*
 * Heartbeat endpoint middleware useful to setting up a path like
 * `/ping` that load balancer or uptime testing external services
 * can make a request before hitting any routes.
 */
func Heartbeat() echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {

		return func(c echo.Context) (err error) {

			request := c.Request()

			if request.Method == "GET" && strings.EqualFold(request.URL.Path, "/ping") {
				projectName := viper.GetString("name")
				return c.JSON(http.StatusOK, map[string]interface{}{
					"message": projectName + ` 🚀  pong`,
				})
			}

			return next(c)
		}
	}
}
