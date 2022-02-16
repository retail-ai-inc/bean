/**#bean*/ /*#bean.replace({{ .Copyright }})**/
// Safe way to execute `go routine` without crashing the parent process while having a `panic`.
package async

import (
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/framework/options"
)

type Task func(c echo.Context)

// `Execute` provides a safe way to execute a function asynchronously, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func Execute(fn Task, e *echo.Echo) {
	go func() {
		c := e.AcquireContext() // Acquire a context from echo.
		c.Reset(nil, nil)       // IMPORTANT: It must be reset before use.
		defer recoverPanic(c)
		fn(c)
	}()
}

// Recover the panic and send the exception to sentry.
func recoverPanic(c echo.Context) {
	if err := recover(); err != nil {
		// Create a new Hub by cloning the existing one.
		if options.SentryOn {
			localHub := sentry.CurrentHub().Clone()
			localHub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("goroutine", "true")
			})
			localHub.Recover(err)
		}
		c.Logger().Error(err)
	}

	// Release the acquired context.
	c.Echo().ReleaseContext(c)
}
