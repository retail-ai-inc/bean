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
	"regexp"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2"
	"github.com/retail-ai-inc/bean/v2/helpers"
	"github.com/retail-ai-inc/bean/v2/internal/gopool"
	"github.com/spf13/viper"
)

type (
	Task        func(c echo.Context)
	TimeoutTask func(c context.Context) error
)

// `Execute` provides a safe way to execute a function asynchronously without any context, recovering if they panic
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

// `ExecuteWithContext` provides a safe way to execute a function asynchronously with a context, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func ExecuteWithContext(fn Task, c echo.Context, poolName ...string) {
	// Acquire a context from echo.
	ec := c.Echo().AcquireContext()

	// IMPORTANT: Must reset before use.
	ec.Reset(c.Request().WithContext(context.TODO()), nil)

	Execute(func() {

		// IMPORTANT - Set the sentry hub key into the context so that `SentryCaptureException` and `SentryCaptureMessage`
		// can pull the right hub and send the exception message to sentry.
		if viper.GetBool("sentry.on") {
			ctx := ec.Request().Context()
			hub := sentry.GetHubFromContext(ctx)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
			}

			hub.Scope().SetRequest(ec.Request())
			ctx = sentry.SetHubOnContext(ctx, hub)

			if helpers.FloatInRange(viper.GetFloat64("sentry.tracesSampleRate"), 0.0, 1.0) > 0.0 {
				path := ec.Request().URL.Path

				span := sentry.StartSpan(ctx, "http",
					sentry.TransactionName(fmt.Sprintf("%s %s ASYNC", ec.Request().Method, path)),
					sentry.ContinueFromRequest(ec.Request()),
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
				r := ec.Request().WithContext(span.Context())
				ec.SetRequest(r)
			}
		}

		// Release the acquired context. This defer will be executed second.
		defer ec.Echo().ReleaseContext(ec)

		// This defer will be executed first.
		defer recoverPanic(ec.Request().Context())

		fn(ec)
	}, poolName...)
}

func ExecuteWithTimeout(ctx context.Context, duration time.Duration, fn TimeoutTask, poolName ...string) {
	hub := sentry.GetHubFromContext(ctx)

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
		sentryOn := viper.GetBool("sentry.on")
		if sentryOn {
			if helpers.FloatInRange(viper.GetFloat64("sentry.tracesSampleRate"), 0.0, 1.0) > 0.0 {
				span := sentry.StartSpan(c, "http",
					sentry.TransactionName(fmt.Sprintf("%s ASYNC", hub.Scope().Transaction())))
				span.Description = helpers.CurrFuncName()

				defer span.Finish()
				c = span.Context()
			}
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

	if !viper.GetBool("sentry.on") {
		bean.Logger().Error(err)
		return
	}

	if c != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentry.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CurrentHub().Clone().CaptureMessage("async context is missing hub information")
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
		if viper.GetBool("sentry.on") {
			localHub := sentry.CurrentHub().Clone()

			if c != nil {
				hub := sentry.GetHubFromContext(c)
				if hub != nil {
					localHub = hub.Clone()
				}
			}

			localHub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("goroutine", "true")
			})

			localHub.Recover(err)
		}

		bean.Logger().Error(err)
	}
}
