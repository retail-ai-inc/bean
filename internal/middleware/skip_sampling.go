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

	"github.com/retail-ai-inc/bean/v2/internal/regex"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/helpers"
)

// SkipSampling skips the sampling of the transaction if the request path is in the skip list.
var SkipSampling = func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		span := sentryecho.GetSpanFromContext(c)
		if span == nil {
			// Should not happen due to the sentryecho middleware's handler, but just in case.
			// https://github.com/getsentry/sentry-go/blob/v0.29.1/echo/sentryecho.go#L39-L113
			ctx := c.Request().Context()
			hub := sentryecho.GetHubFromContext(c)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
			}
			hub.Scope().SetRequest(c.Request())
			ctx = sentry.SetHubOnContext(ctx, hub)
			path := c.Request().URL.Path
			// Start a sentry span for tracing.
			span = sentry.StartTransaction(ctx, fmt.Sprintf("%s %s", c.Request().Method, path),
				sentry.WithOpName("http"),
				sentry.WithDescription(helpers.CurrFuncName()),
				sentry.ContinueFromRequest(c.Request()),
			)
			defer span.Finish()
		}

		path := c.Request().URL.Path
		if regex.SkipSampling(path) {
			span.Sampled = sentry.SampledFalse
		}

		r := c.Request().WithContext(span.Context())
		c.SetRequest(r)
		return next(c)
	}
}
