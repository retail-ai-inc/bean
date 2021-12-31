/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package middleware

import (
	"net/http"
	"sort"
	"strings"

	ierror /**#bean*/ "demo/framework/internals/error" /*#bean.replace("{{ .PkgPath }}/framework/internals/error")**/

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

// MethodNotAllowedAndRouteNotFound middleware reply HTTP 405 if a wrong method been called for an API route.
// And 404 for wrong route page.
func MethodNotAllowedAndRouteNotFound() echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {

		return func(c echo.Context) (err error) {

			allowedMethod := viper.GetStringSlice("http.allowedMethod")

			isRouteMatched := false
			isMethodMatched := false

			for _, r := range c.Echo().Routes() {

				// XXX: IMPORTANT - Just ignore unnecessary system route
				if strings.Contains(r.Name, "glob..func1") {
					continue
				}

				// XXX: IMPORTANT - `allowedMethod` has to be a sorted slice.
				i := sort.SearchStrings(allowedMethod, r.Method)
				if i >= len(allowedMethod) || allowedMethod[i] != r.Method {
					continue
				}

				// XXX: IMPORTANT - c.Path() contains the actual registered route like `/user/:id/profile`
				if r.Path == c.Path() && r.Method != c.Request().Method {
					isRouteMatched = true
					isMethodMatched = false

				} else if r.Path == c.Path() && r.Method == c.Request().Method {
					isRouteMatched = true
					isMethodMatched = true

					// If both path and method get matched then we don't need to continue the loop any more.
					break
				}
			}

			if !isRouteMatched {

				// XXX: IMPORTANT - Following 2 routes are special purpose and implemented in `master.go` as a middleware.
				// As this 2 routes implemented as middleware that's why they are not listed under `echo` route list.
				if (c.Path() == "/ping" || c.Path() == "/route/stats") && c.Request().Method == "GET" {
					return next(c)

				} else {
					return c.JSON(http.StatusNotFound, map[string]interface{}{
						"errorCode": ierror.RESOURCE_NOT_FOUND,
						"errors":    nil,
					})
				}

			} else if isRouteMatched && !isMethodMatched {
				return c.JSON(http.StatusMethodNotAllowed, map[string]interface{}{
					"errorCode": ierror.METHOD_NOT_ALLOWED,
					"errors":    nil,
				})

			} else if isRouteMatched && isMethodMatched {
				return next(c)
			}

			return next(c)
		}
	}
}
