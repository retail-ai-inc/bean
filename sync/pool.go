// Package sync provides ways to execute multiple tasks concurrently
// while synchronously managing them at the same time.
package sync

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v2"
	"github.com/retail-ai-inc/bean/v2/internal/regex"
	"github.com/sourcegraph/conc/panics"
	"github.com/sourcegraph/conc/pool"
)

// Pool provides a way to execute multiple tasks concurrently and synchronously wait for all of them to finish.
// It also recovers from panics within the tasks and support sentry tracing.
type Pool struct {
	pool *pool.ContextPool
	span *sentry.Span
}

// NewPool creates a new pool with the given context and options.
func NewPool(ctx context.Context, opts ...PoolOption) *Pool {

	// set default options
	plOpts := &poolOptions{
		req: &http.Request{
			Method: "",
			URL:    &url.URL{},
		},
		max: 0,
	}

	for _, opt := range opts {
		opt(plOpts)
	}

	// set setry transaction or span
	var span *sentry.Span
	if bean.BeanConfig.Sentry.On {
		hub := sentry.GetHubFromContext(ctx)
		if hub == nil {
			hub = sentry.CurrentHub().Clone()
		}

		hub.Scope().SetRequest(plOpts.req)
		ctx = sentry.SetHubOnContext(ctx, hub)

		if bean.BeanConfig.Sentry.TracesSampleRate > 0.0 {
			urlPath := plOpts.req.URL.Path

			functionName := "unknown function"
			if bean.BeanConfig.Sentry.On && bean.BeanConfig.Sentry.TracesSampleRate > 0.0 {
				if pc, file, line, ok := runtime.Caller(1); ok {
					functionName = fmt.Sprintf("%s:%d\n\t\r %s\n", path.Base(file), line, runtime.FuncForPC(pc).Name())
				}
			}
			span = sentry.StartSpan(ctx, "sync",
				sentry.WithTransactionName(fmt.Sprintf("%s %s SYNC", plOpts.req.Method, urlPath)),
				sentry.ContinueFromRequest(plOpts.req),
				sentry.WithDescription(functionName),
			)

			if regex.MatchAnyTraceSkipPath(urlPath) {
				span.Sampled = sentry.SampledFalse
			}
		}
	}

	if span != nil {
		ctx = span.Context()
	}

	var pl *pool.ContextPool
	if plOpts.cancelOnFirstErr {
		pl = pool.New().WithContext(ctx).WithFirstError().WithCancelOnError()
	} else {
		pl = pool.New().WithErrors().WithContext(ctx)
	}
	if plOpts.max > 0 {
		pl.WithMaxGoroutines(plOpts.max)
	}

	return &Pool{
		pool: pl,
		span: span,
	}
}

// PoolOption provides options to configure the pool.
type PoolOption func(*poolOptions)

type poolOptions struct {
	max              int
	req              *http.Request
	cancelOnFirstErr bool
}

// WithRequest sets a http request for sentry tracing.
func WithRequest(req *http.Request) PoolOption {
	return func(opts *poolOptions) {
		opts.req = req
	}
}

// WithMaxGoroutines sets the maximum number of goroutines that can be executed concurrently.
func WithMaxGoroutines(max int) PoolOption {
	return func(opts *poolOptions) {
		opts.max = max
	}
}

// WithCancelOnFirstErr cancels all tasks when the first error occurs.
func WithCancelOnFirstErr() PoolOption {
	return func(opts *poolOptions) {
		opts.cancelOnFirstErr = true
	}
}

// Go executes a task concurrently.
// It return a panic event as an error, not a panic if any.
func (p *Pool) Go(f func(ctx context.Context) error) {
	p.pool.Go(func(ctx context.Context) (err error) {
		// recover the panic
		var catcher panics.Catcher
		defer func() {
			if r := catcher.Recovered(); r != nil {
				err = r.AsError()

				if bean.BeanConfig.Sentry.On {
					var localHub *sentry.Hub
					if ctx != nil {
						localHub = sentry.GetHubFromContext(ctx)
					}
					if localHub == nil {
						localHub = sentry.CurrentHub().Clone()
					}
					localHub.ConfigureScope(func(scope *sentry.Scope) {
						scope.SetTag("goroutine", "true")
					})
					localHub.Recover(err)
				}
			}
		}()

		// execute the task and catch panic if any
		catcher.Try(func() {
			err = f(ctx)
		})

		if err != nil {
			return err
		}

		return nil
	})
}

// Wait waits for all tasks to finish and get an error (or multi joined errors) if any.
func (p *Pool) Wait() error {
	defer func() {
		if p.span != nil {
			p.span.Finish()
		}
	}()

	return p.pool.Wait()
}
