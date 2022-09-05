// Copyright The RAI Inc.
// The RAI Authors

// Safe way to execute `go routine` without crashing the parent process while having a `panic`.
package async

import (
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean"
)

type Task func(c echo.Context)

// `Execute` provides a safe way to execute a function asynchronously without any context, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func Execute(fn func()) {
	go func() {
		defer recoverPanic(nil)
		fn()
	}()
}

// `ExecuteWithContext` provides a safe way to execute a function asynchronously with a context, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func ExecuteWithContext(fn Task, c echo.Context) {
	// Acquire a context from echo.
	ctx := c.Echo().AcquireContext()

	// IMPORTANT: Must reset before use.
	ctx.Reset(c.Request(), nil)

	go func() {
		// Release the acquired context. This defer will be execute second.
		defer ctx.Echo().ReleaseContext(ctx)

		// This defer will be executed first.
		defer recoverPanic(ctx)
		fn(ctx)
	}()
}

// Recover the panic and send the exception to sentry.
func recoverPanic(c echo.Context) {
	if err := recover(); err != nil {
		// Create a new Hub by cloning the existing one.
		if bean.SentryOn {
			localHub := sentry.CurrentHub().Clone()

			if c != nil {
				localHub.Scope().SetRequest(c.Request())
			}

			localHub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("goroutine", "true")
			})

			localHub.Recover(err)
		}

		bean.Logger().Error(err)
	}
}
