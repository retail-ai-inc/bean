// Copyright 2020 The RAI Inc.
// The RAI Authors
package middleware

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestTimeout attach a timeout context to the request.
func RequestTimeout(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			timeoutCtx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			c.SetRequest(c.Request().WithContext(timeoutCtx))
			defer cancel()
			return next(c)
		}
	}
}
