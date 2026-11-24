package middleware

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/labstack/echo/v5"
	berror "github.com/retail-ai-inc/bean/v3/error"
	"github.com/retail-ai-inc/bean/v3/stacktrace"
)

const maxPanicStackDepth = 64

// Recover recovers from panics in the handler chain and returns them as `*berror.PanicError`, which carries
// the stack trace of the panic site, so the error handlers can log / report it to Sentry with the right stack.
// `http.ErrAbortHandler` is re-panicked, as `net/http` expects.
func Recover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if r == http.ErrAbortHandler { //nolint:errorlint // the sentinel is compared as documented by net/http
						panic(r)
					}
					err = &berror.PanicError{Value: r, Stack: panicStack()}
				}
			}()
			return next(c)
		}
	}
}

// panicStack returns the stack of the panicking goroutine, starting at the panic site. It must be called
// from the deferred function that recovered the panic: at that point the frames of the panicking function
// are still on the stack, below the runtime's panic machinery.
func panicStack() *stacktrace.Stack {
	pcs := make([]uintptr, maxPanicStackDepth)
	pcs = pcs[:runtime.Callers(1, pcs)]

	// Drop everything up to `runtime.gopanic` (this function, the deferred function), then the runtime frames
	// leading to the panic (e.g. `runtime.panicmem`, `runtime.sigpanic` for a nil pointer dereference).
	for i, pc := range pcs {
		if fn := runtime.FuncForPC(pc - 1); fn != nil && fn.Name() == "runtime.gopanic" {
			pcs = pcs[i+1:]
			break
		}
	}
	for len(pcs) > 0 {
		fn := runtime.FuncForPC(pcs[0] - 1)
		if fn == nil || !strings.HasPrefix(fn.Name(), "runtime.") {
			break
		}
		pcs = pcs[1:]
	}

	st := stacktrace.Stack(pcs)
	return &st
}
