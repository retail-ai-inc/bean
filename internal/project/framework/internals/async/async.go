/**#bean*/ /*#bean.replace({{ .Copyright }})**/
// Safe way to execute `go routine` without crashing the parent process while having a `panic`.
package async

import (
	/**#bean*/
	"demo/framework/internals/sentry"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/sentry")**/

	"github.com/labstack/echo/v4"
)

type Task func(c echo.Context)

// `Execute` provides a safe way to execute a function asynchronously, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func Execute(fn Task, e *echo.Echo) {

	go func() {
		// Acquire a context from global echo instance and reset it to avoid race condition.
		c := e.AcquireContext()
		c.Reset(nil, nil)

		defer recoverPanic(c)
		fn(c)
	}()
}

// Write the error to console or sentry when a goroutine of a task panics.
func recoverPanic(c echo.Context) {

	if r := recover(); r != nil {
		sentry.PushData(c, r, nil, false)
	}

	// Release the acquired context.
	c.Echo().ReleaseContext(c)
}
