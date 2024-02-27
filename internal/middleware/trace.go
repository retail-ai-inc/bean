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
	"regexp"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/helpers"
	"github.com/spf13/viper"
)

// Tracer attach a root sentry span context to the request.
func Tracer() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var ctx = c.Request().Context()
			hub := sentry.GetHubFromContext(ctx)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
				ctx = sentry.SetHubOnContext(ctx, hub)
			}
			path := c.Request().URL.Path
			// Start a sentry span for tracing.
			span := sentry.StartSpan(ctx, "http",
				sentry.WithTransactionName(fmt.Sprintf("%s %s", c.Request().Method, path)),
				sentry.ContinueFromRequest(c.Request()),
			)
			span.Description = helpers.CurrFuncName()

			// If `skipTracesEndpoints` has some path(s) then let's skip performance sample for those URI.
			skipTracesEndpoints := viper.GetStringSlice("sentry.skipTracesEndpoints")
			for _, endpoint := range skipTracesEndpoints {
				if regexp.MustCompile(endpoint).MatchString(path) {
					span.Sampled = sentry.SampledFalse
					break
				}
			}

			defer span.Finish()
			r := c.Request().WithContext(span.Context())
			c.SetRequest(r)
			return next(c)
		}
	}
}
