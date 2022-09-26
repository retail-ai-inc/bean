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
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/helpers"
	bstring "github.com/retail-ai-inc/bean/string"
	"github.com/spf13/viper"
)

// Tracer attach a root sentry span context to the request.
func Tracer() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Start a sentry span for tracing.
			span := sentry.StartSpan(c.Request().Context(), "http",
				sentry.TransactionName(fmt.Sprintf(c.Request().RequestURI)),
				sentry.ContinueFromRequest(c.Request()),
			)
			span.Description = helpers.CurrFuncName()

			// If `skipTracesEndpoints` has some path(s) then let's skip performance sample for those URI.
			skipTracesEndpoints := viper.GetStringSlice("sentry.skipTracesEndpoints")

			if len(skipTracesEndpoints) > 0 {
				path := c.Request().URL.Path
				if path == "/" { // To avoid index matching for any URI path.
					if bstring.Contains(skipTracesEndpoints, path) {
						span.Sampled = sentry.SampledFalse
					}
				} else {
					_, matches := bstring.MatchAllSubstringsInAString(path, skipTracesEndpoints...)
					if matches > 0 {
						span.Sampled = sentry.SampledFalse
					}
				}
			}

			defer span.Finish()
			r := c.Request().Clone(span.Context())
			c.SetRequest(r)
			return next(c)
		}
	}
}
