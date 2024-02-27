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

// Safe way to execute `go routine` without crashing the parent process while having a `panic`.
package async

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"runtime"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2"
	"github.com/retail-ai-inc/bean/v2/internal/gopool"
)

type (
	Task        func(c context.Context)
	TimeoutTask func(c context.Context) error
)

// Execute provides a safe way to execute a function asynchronously without any context, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func Execute(fn func(), poolName ...string) {
	var asyncFunc = func(task func()) {
		go func() {
			defer recoverPanic(context.TODO())
			task()
		}()
	}

	if len(poolName) > 0 {
		pool, err := gopool.GetPool(poolName[0])
		if err == nil && pool != nil {
			asyncFunc = func(task func()) {
				defer recoverPanic(context.TODO())
				err = pool.Submit(task)
				if err != nil {
					panic(err)
				}
			}
		} else {
			bean.Logger().Warnf("async func will execute without goroutine pool, the pool name is %q\n", poolName[0])
		}
	}

	asyncFunc(fn)
}

// ExecuteWithContext provides a safe way to execute a function asynchronously with a context, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func ExecuteWithContext(fn Task, c echo.Context, poolName ...string) {
	functionName := "unknown function"
	if bean.BeanConfig.Sentry.On && bean.BeanConfig.Sentry.TracesSampleRate > 0.0 {
		if pc, file, line, ok := runtime.Caller(1); ok {
			functionName = fmt.Sprintf("%s:%d\n\t\r %s\n", path.Base(file), line, runtime.FuncForPC(pc).Name())
		}
	}

	// Acquire a context from echo.
	ec := c.Echo().AcquireContext()

	// IMPORTANT: Must reset before use.
	ec.Reset(c.Request().WithContext(context.TODO()), nil)

	Execute(func() {
		ctx := ec.Request().Context()
		// IMPORTANT - Set the sentry hub key into the context so that `SentryCaptureException` and `SentryCaptureMessage`
		// can pull the right hub and send the exception message to sentry.
		if bean.BeanConfig.Sentry.On {
			hub := sentry.GetHubFromContext(ctx)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
			}

			hub.Scope().SetRequest(ec.Request())
			ctx = sentry.SetHubOnContext(ctx, hub)

			if bean.BeanConfig.Sentry.TracesSampleRate > 0.0 {
				urlPath := ec.Request().URL.Path

				span := sentry.StartSpan(ctx, "async",
					sentry.WithTransactionName(fmt.Sprintf("%s %s ASYNC", ec.Request().Method, urlPath)),
					sentry.ContinueFromRequest(ec.Request()),
				)

				span.Description = functionName

				// If `skipTracesEndpoints` has some path(s) then let's skip performance sample for those URI.
				skipTracesEndpoints := bean.BeanConfig.Sentry.SkipTracesEndpoints

				for _, endpoint := range skipTracesEndpoints {
					if regexp.MustCompile(endpoint).MatchString(urlPath) {
						span.Sampled = sentry.SampledFalse
						break
					}
				}

				defer span.Finish()
				ctx = span.Context()
			}
		}

		// Release the acquired context. This defer will be executed second.
		defer c.Echo().ReleaseContext(ec)

		// This defer will be executed first.
		defer recoverPanic(ctx)

		fn(ctx)
	}, poolName...)
}

func ExecuteWithTimeout(ctx context.Context, duration time.Duration, fn TimeoutTask, poolName ...string) {
	functionName := "unknown function"
	if bean.BeanConfig.Sentry.On && bean.BeanConfig.Sentry.TracesSampleRate > 0.0 {
		if pc, file, line, ok := runtime.Caller(1); ok {
			functionName = fmt.Sprintf("%s:%d\n\t\r %s\n", path.Base(file), line, runtime.FuncForPC(pc).Name())
		}
	}

	hub := sentry.GetHubFromContext(ctx)
	parentSpan := sentry.SpanFromContext(ctx)

	Execute(func() {
		var (
			c      context.Context
			cancel context.CancelFunc
		)
		if duration <= 0 {
			c = context.TODO()
		} else {
			c, cancel = context.WithTimeout(context.TODO(), duration)
			defer cancel()
		}

		if hub == nil {
			hub = sentry.CurrentHub().Clone()
			c = sentry.SetHubOnContext(c, hub)
		} else {
			c = sentry.SetHubOnContext(c, hub)
		}

		// can pull the right hub and send the exception message to sentry.
		if bean.BeanConfig.Sentry.On && bean.BeanConfig.Sentry.TracesSampleRate > 0.0 {
			var transactionName string
			if parentSpan != nil {
				transactionName = parentSpan.Name
			}

			span := sentry.StartSpan(c, "async",
				sentry.WithTransactionName(fmt.Sprintf("%s ASYNC", transactionName)))

			span.Description = functionName

			defer span.Finish()
			c = span.Context()
		}

		// This defer will be executed first.
		defer recoverPanic(c)

		CaptureException(c, fn(c))
	}, poolName...)
}

func CaptureException(c context.Context, err error) {
	if err == nil {
		return
	}

	if !bean.BeanConfig.Sentry.On {
		bean.Logger().Error(err)
		return
	}

	if c != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentry.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CurrentHub().Clone().CaptureException(fmt.Errorf("async context is missing hub information: %w", err))
		}
		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureException(err)
}

// Recover the panic and send the exception to sentry.
func recoverPanic(c context.Context) {
	if err := recover(); err != nil {
		// Create a new Hub by cloning the existing one.
		if bean.BeanConfig.Sentry.On {
			var localHub *sentry.Hub

			if c != nil {
				localHub = sentry.GetHubFromContext(c)
			}

			if localHub == nil {
				localHub = sentry.CurrentHub().Clone()
			}

			localHub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("goroutine", "true")
			})

			localHub.Recover(err)
		}

		bean.Logger().Error(err)
	}
}
