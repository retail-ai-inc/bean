// Copyright The RAI Inc.
// The RAI Authors
package middleware

import (
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/framework/internals/helpers"
	"github.com/retail-ai-inc/bean/framework/options"
)

// ServerHeader middleware adds a `Server` header to the response.
func ServerHeader(name, version string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			// Start a sentry span for tracing.
			if options.SentryOn {
				span := sentry.StartSpan(c.Request().Context(), "middleware")
				span.Description = helpers.CurrFuncName()
				defer span.Finish()
			}
			c.Response().Header().Set(echo.HeaderServer, name+"/"+version)
			return next(c)
		}
	}
}
