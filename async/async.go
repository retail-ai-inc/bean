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
	"errors"
	"fmt"
	"path"
	"runtime"
	"time"

	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/retail-ai-inc/bean/v2/internal/gopool"
	"github.com/retail-ai-inc/bean/v2/internal/regex"
	"github.com/retail-ai-inc/bean/v2/log"
	"github.com/retail-ai-inc/bean/v2/trace"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/panjf2000/ants/v2"
	"github.com/retail-ai-inc/bean/v2/config"
)

type (
	Task        func(c context.Context)
	TaskWithCtx func(c context.Context) error
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

	if len(poolName) > 0 && poolName[0] != "" {
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
			log.Logger().Warnf("async func will execute without goroutine pool, the pool name is %q\n", poolName[0])
		}
	}

	asyncFunc(fn)
}

// ExecuteWithContext provides a safe way to execute a function asynchronously with a context, recovering if they panic
// and provides all error stack aiming to facilitate fail causes discovery.
func ExecuteWithContext(fn Task, c echo.Context, poolName ...string) {
	functionName := "unknown function"
	if config.Bean.Sentry.On && config.Bean.Sentry.TracesSampleRate > 0.0 {
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
		if config.Bean.Sentry.On {
			hub := sentry.GetHubFromContext(ctx)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
			}

			hub.Scope().SetRequest(ec.Request())
			ctx = sentry.SetHubOnContext(ctx, hub)

			if config.Bean.Sentry.TracesSampleRate > 0.0 {
				urlPath := ec.Request().URL.Path

				span := sentry.StartSpan(ctx, "async",
					sentry.WithTransactionName(fmt.Sprintf("%s %s ASYNC", ec.Request().Method, urlPath)),
					sentry.ContinueFromRequest(ec.Request()),
				)

				span.Description = functionName

				if regex.SkipSampling(urlPath) {
					span.Sampled = sentry.SampledFalse
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

func ExecuteWithTimeout(ctx context.Context, duration time.Duration, fn TaskWithCtx, poolName ...string) {
	functionName := "unknown function"
	if config.Bean.Sentry.On && config.Bean.Sentry.TracesSampleRate > 0.0 {
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
		if config.Bean.Sentry.On && config.Bean.Sentry.TracesSampleRate > 0.0 {
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
	trace.SentryCaptureException(c, err)
}

type AsyncOption func(*asyncOptions)

// WithPoolName sets the pool name for the async task.
// If the pool name is not provided, the default pool will be used.
func WithPoolName(poolName string) AsyncOption {
	return func(o *asyncOptions) {
		o.poolName = poolName
	}
}

// WithTimeout sets the timeout for the async task.
// If the timeout is not provided, the task will run without a timeout.
func WithTimeout(d time.Duration) AsyncOption {
	return func(o *asyncOptions) {
		o.timeout = d
	}
}

type asyncOptions struct {
	poolName string
	timeout  time.Duration
}

// ExecuteWithContext execute a function returning an error asynchronously
// with a starndard context (not echo context), recovering if they panic.
func ExecuteContext(ctx context.Context, fn TaskWithCtx, asyncOpts ...AsyncOption) error {

	// by default
	opts := &asyncOptions{
		poolName: "",
		timeout:  0,
	}

	for _, apply := range asyncOpts {
		apply(opts)
	}

	newCtx, cancel := newContext(ctx, config.Bean.Sentry.On, opts.timeout)

	var sentrySamplingOpts []sentry.SpanOption
	if config.Bean.Sentry.On && config.Bean.Sentry.TracesSampleRate > 0.0 {
		sentrySamplingOpts = setupSentrySampling(ctx)
	}

	// Define the task to be executed.
	task := newTask(newCtx, fn, cancel, sentrySamplingOpts...)

	// Actually execute the task with the pool name if provided.
	var err error
	if opts.poolName != "" {
		err = execute(newCtx, task, withPool(opts.poolName))
	} else {
		err = execute(newCtx, task)
	}
	if err != nil {
		return err
	}

	return nil
}

var ErrTaskTimeout = fmt.Errorf("task execution timeout")

func newContext(current context.Context, sentryOn bool, timeout time.Duration) (context.Context, context.CancelFunc) {

	new := context.Background()

	if sentryOn {
		// Set scope to the hub.
		hub := sentry.GetHubFromContext(current)
		if hub == nil {
			hub = sentry.CurrentHub()
		}
		clone := hub.Clone()

		req, reqFound := bctx.GetRequest(current)
		if reqFound {
			clone.Scope().SetRequest(req)
		}
		new = sentry.SetHubOnContext(new, clone)
	}

	if reqID, ok := bctx.GetRequestID(current); ok {
		new = bctx.SetRequestID(new, reqID)
	}

	// Set the timeout to the context.
	var cancel context.CancelFunc = func() {} // do nothing
	if timeout > 0 {
		new, cancel = context.WithTimeoutCause(new, timeout, ErrTaskTimeout)
	}

	return new, cancel
}

func setupSentrySampling(current context.Context) []sentry.SpanOption {

	opts := make([]sentry.SpanOption, 0, 3) // continue from headers, description, transaction name at least

	// Continue the trace by passing the sentry-trace id, not by sharing the same span object, like distributed tracing across different servers.
	// This is because the same span in the context is used in multiple goroutines, which causes a data race issue.
	opts = append(opts, sentry.ContinueFromHeaders(extractTracing(current)))

	var (
		description     string
		transactionName string
	)

	span := sentry.SpanFromContext(current)
	if span != nil {
		description = span.Description
		transactionName = span.Name
	}
	if description == "" {
		if pc, file, line, ok := runtime.Caller(2); ok {
			description = fmt.Sprintf("%s:%d\n\t\r %s\n", path.Base(file), line, runtime.FuncForPC(pc).Name())
		}
	}
	opts = append(opts, sentry.WithDescription(fmt.Sprintf("%s ASYNC", description)))

	req, reqFound := bctx.GetRequest(current)
	if reqFound {
		path := req.URL.Path
		sampled := func() sentry.Sampled {
			if regex.SkipSampling(path) {
				return sentry.SampledFalse
			}
			return sentry.SampledUndefined
		}()
		opts = append(opts, sentry.WithSpanSampled(sampled))

		if transactionName == "" {
			transactionName = fmt.Sprintf("%s %s", req.Method, path)
		}
	}

	opts = append(opts, sentry.WithTransactionName(fmt.Sprintf("%s ASYNC", transactionName)))

	return opts
}

func extractTracing(ctx context.Context) (sentryTrace, baggage string) {

	span := sentry.SpanFromContext(ctx)
	if span == nil {
		return "", ""
	}

	return span.ToSentryTrace(), span.ToBaggage()
}

func newTask(ctx context.Context, fn TaskWithCtx,
	cancelFunc context.CancelFunc, sentryOpts ...sentry.SpanOption,
) func() {

	cancel := func() {}
	if cancelFunc != nil {
		cancel = cancelFunc
	}

	return func() {
		defer cancel()

		if len(sentryOpts) > 0 {
			// Start a new span with a new context to avoid data race
			// when the same span in the same context is used in multiple goroutines.
			span := sentry.StartSpan(ctx, "async", sentryOpts...)
			defer span.Finish()
			ctx = span.Context()
		}

		defer recoverPanic(ctx)
		// Run the task with the context.
		err := fn(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) && context.Cause(ctx) == ErrTaskTimeout {
				err = errors.Join(ErrTaskTimeout, err)
			}
			trace.SentryCaptureException(ctx, err)
			msg := map[string]interface{}{
				"message": "Async task failed.",
				"error":   err.Error(),
			}
			if reqID, ok := bctx.GetRequestID(ctx); ok {
				msg["request_id"] = reqID
			}
			log.Logger().Errorj(msg)
			return
		}
	}
}

type execOptions struct {
	poolName *string
}

type execOption func(*execOptions)

func withPool(name string) execOption {
	return func(o *execOptions) {
		o.poolName = &name
	}
}

// execute runs a task asynchronously and with a default pool if no pool is provided.
// It returns an error if the task cannot be submitted to the pool (not related to the outcome of the task).
func execute(ctx context.Context, task func(), execOpts ...execOption) error {

	opts := &execOptions{
		poolName: nil,
	}

	for _, apply := range execOpts {
		apply(opts)
	}

	var pool *ants.Pool
	if opts.poolName != nil {
		if got, err := gopool.GetPool(*opts.poolName); err == nil && got != nil {
			pool = got
		} else {
			log.Logger().Errorf("pool(%s) not found, using default pool", *opts.poolName)
		}
	}
	if pool == nil {
		pool = gopool.GetDefaultPool()
	}

	submitToPool := func(task func()) error {
		defer recoverPanic(ctx)
		err := pool.Submit(task)
		if err != nil {
			return err
		}
		return nil
	}

	return submitToPool(task)
}

// recover the panic and send the exception to sentry.
func recoverPanic(ctx context.Context) {
	if v := recover(); v != nil {
		// Create a new Hub by cloning the existing one.
		if config.Bean.Sentry.On {
			var localHub *sentry.Hub

			if ctx != nil {
				localHub = sentry.GetHubFromContext(ctx)
			}

			if localHub == nil {
				localHub = sentry.CurrentHub()
			}
			clone := localHub.Clone()

			clone.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("goroutine", "true")
			})

			clone.Recover(v)
		}

		msg := map[string]interface{}{
			"message": "Recovered panic from async task.",
			"cause":   v,
		}
		if reqID, ok := bctx.GetRequestID(ctx); ok {
			msg["request_id"] = reqID
		}
		log.Logger().Errorj(msg)
	}
}
