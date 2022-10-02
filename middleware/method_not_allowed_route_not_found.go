// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package middleware

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	berror "github.com/retail-ai-inc/bean/error"
	broute "github.com/retail-ai-inc/bean/route"
	"github.com/spf13/viper"
)

var sortedAllowedMethodSlice []string
var sortAllowedMethodOnce sync.Once

// MethodNotAllowedAndRouteNotFound middleware will reply HTTP 405 if a wrong method been called for an API route.
// This middleware will also return 404 if a page doesn't exist.
func MethodNotAllowedAndRouteNotFound() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			allowedMethod := sortedAllowedMethod()
			isRouteMatched := false
			isMethodMatched := false

			for _, r := range broute.Routes {
				// IMPORTANT - Just ignore unnecessary system route
				if strings.Contains(r.Name, "glob..func1") {
					continue
				}

				// IMPORTANT - `allowedMethod` has to be a sorted slice.
				i := sort.SearchStrings(allowedMethod, r.Method)
				if i >= len(allowedMethod) || allowedMethod[i] != r.Method {
					continue
				}

				// Match the request path with registered routes.
				_, ok := r.PathSegment.Match(c.Request().URL.Path)

				// IMPORTANT - c.Path() contains the actual registered route like `/user/:id/profile`
				if ok && r.Method != c.Request().Method {
					isRouteMatched = true
					isMethodMatched = false

				} else if ok && r.Method == c.Request().Method {
					isRouteMatched = true
					isMethodMatched = true

					// If both path and method get matched then we don't need to continue the loop any more.
					break
				}
			}

			if !isRouteMatched {
				if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
					return c.Render(http.StatusNotFound, "errors/html/404", echo.Map{"stacktrace": nil})
				} else {
					return c.JSON(http.StatusNotFound, map[string]interface{}{
						"errorCode": berror.RESOURCE_NOT_FOUND,
						"errors":    nil,
					})
				}

			} else if isRouteMatched && !isMethodMatched {

				if !strings.Contains(c.Request().Header.Get("Content-Type"), "application/json") {
					return c.Render(http.StatusMethodNotAllowed, "errors/html/405", echo.Map{"stacktrace": nil})
				} else {
					return c.JSON(http.StatusMethodNotAllowed, map[string]interface{}{
						"errorCode": berror.METHOD_NOT_ALLOWED,
						"errors":    nil,
					})
				}
			}

			return next(c)
		}
	}
}

func sortedAllowedMethod() []string {
	sortAllowedMethodOnce.Do(func() {
		sortedAllowedMethodSlice = viper.GetStringSlice("http.allowedMethod")
		sort.Strings(sortedAllowedMethodSlice)
	})

	return sortedAllowedMethodSlice
}
