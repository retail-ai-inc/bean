// Copyright The RAI Inc.
// The RAI Authors

// Safe way to execute `go routine` without crashing the parent process while having a `panic`.
package async

import (
	"context"
	"fmt"
	"regexp"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean"
	"github.com/retail-ai-inc/bean/gopool"
	"github.com/retail-ai-inc/bean/helpers"
	"github.com/spf13/viper"
)

type Task func(c echo.Context)

// `Execute` provides a safe way to execute a function asynchronously without any context, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func Execute(fn func(), poolName ...string) {
	var asyncFunc = func(task func()) {
		go func() {
			defer recoverPanic(nil)
			task()
		}()
	}
	if len(poolName) > 0 {
		pool, err := gopool.GetPool(poolName[0])
		if err == nil && pool != nil {
			asyncFunc = func(task func()) {
				defer recoverPanic(nil)
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
		if bean.BeanConfig.Sentry.On {
			ctx := ec.Request().Context()
			hub := sentry.GetHubFromContext(ctx)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
			}
			hub.Scope().SetRequest(ec.Request())
			ctx = sentry.SetHubOnContext(ctx, hub)
			ec.Set(bean.SentryHubContextKey, hub)

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
		defer recoverPanic(ec)

		fn(ec)
	}, poolName...)
}

// Recover the panic and send the exception to sentry.
func recoverPanic(c echo.Context) {
	if err := recover(); err != nil {
		// Create a new Hub by cloning the existing one.
		if bean.BeanConfig.Sentry.On {
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
