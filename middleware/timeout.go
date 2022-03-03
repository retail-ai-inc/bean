// Copyright The RAI Inc.
// The RAI Authors
package middleware

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/helpers"
	"github.com/retail-ai-inc/bean/options"
)

// RequestTimeout attach a timeout context to the request.
func RequestTimeout(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Start a sentry span for tracing.
			if options.SentryOn {
				span := sentry.StartSpan(c.Request().Context(), "http.middleware")
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
