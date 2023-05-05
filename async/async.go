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
	"net/http"
	"net/http/httptest"
	"regexp"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean"
	"github.com/retail-ai-inc/bean/gopool"
	"github.com/retail-ai-inc/bean/helpers"
	"github.com/spf13/viper"
)

type Task func(c context.Context)

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
func ExecuteWithContext(fn Task, ctx context.Context, poolName ...string) {
	if ctx == nil {
		ctx = context.TODO()
	}
	req, ok := ctx.Value(0).(*http.Request)
	if ok {
		// clone request
		req = req.Clone(context.TODO())
	} else {
		req = httptest.NewRequest("", "/", nil)
	}

	Execute(func() {
		// IMPORTANT - Set the sentry hub key into the context so that `SentryCaptureException` and `SentryCaptureMessage`
		// can pull the right hub and send the exception message to sentry.
		if bean.BeanConfig.Sentry.On {
			ctx := req.Context()
			hub := sentry.GetHubFromContext(ctx)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
				ctx = sentry.SetHubOnContext(ctx, hub)
			}

			hub.Scope().SetRequest(req)

			if helpers.FloatInRange(viper.GetFloat64("sentry.tracesSampleRate"), 0.0, 1.0) > 0.0 {
				path := req.URL.Path

				span := sentry.StartSpan(ctx, "http",
					sentry.TransactionName(fmt.Sprintf("%s %s ASYNC", req.Method, path)),
					sentry.ContinueFromRequest(req),
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
				req = req.WithContext(span.Context())
			}
		}

		// This defer will be executed first.
		defer recoverPanic(req)

		fn(req.Context())
	}, poolName...)
}

// Recover the panic and send the exception to sentry.
func recoverPanic(r *http.Request) {
	if err := recover(); err != nil {
		// Create a new Hub by cloning the existing one.
		if bean.BeanConfig.Sentry.On {
			localHub := sentry.CurrentHub().Clone()

			if r != nil {
				localHub.Scope().SetRequest(r)
			}

			localHub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("goroutine", "true")
			})

			localHub.Recover(err)
		}

		bean.Logger().Error(err)
	}
}
