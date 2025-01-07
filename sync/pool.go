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
	"github.com/retail-ai-inc/bean/v2/config"
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

// NewPool creates a new pool with the given context and options.
func NewPool(ctx context.Context, opts ...PoolOption) *Pool {

	// set default options
	plOpts := &poolOptions{
		req: &http.Request{
			Method: "",
			URL:    &url.URL{},
		},
		max:              0,
		cancelOnFirstErr: false,
	}

	for _, opt := range opts {
		opt(plOpts)
	}

	// set sentry transaction or span
	span := setSpan(ctx, plOpts.req)
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

// Go executes a task concurrently.
// It return a panic event as an error, not a panic if any.
func (p *Pool) Go(f func(ctx context.Context) error) {
	p.pool.Go(func(ctx context.Context) (err error) {
		// recover the panic
		var catcher panics.Catcher
		defer func() {
			if r := catcher.Recovered(); r != nil {
				err = r.AsError()
				capturePanic(ctx, err)
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

func setSpan(ctx context.Context, req *http.Request) *sentry.Span {
	var span *sentry.Span

	if config.Bean.Sentry.On {
		hub := sentry.GetHubFromContext(ctx)
		if hub == nil {
			hub = sentry.CurrentHub().Clone()
		}

		hub.Scope().SetRequest(req)
		ctx = sentry.SetHubOnContext(ctx, hub)

		if config.Bean.Sentry.TracesSampleRate > 0.0 {
			urlPath := req.URL.Path

			functionName := "unknown function"
			if config.Bean.Sentry.On && config.Bean.Sentry.TracesSampleRate > 0.0 {
				if pc, file, line, ok := runtime.Caller(1); ok {
					functionName = fmt.Sprintf("%s:%d\n\t\r %s\n", path.Base(file), line, runtime.FuncForPC(pc).Name())
				}
			}
			span = sentry.StartSpan(ctx, "sync",
				sentry.WithTransactionName(fmt.Sprintf("%s %s SYNC", req.Method, urlPath)),
				sentry.ContinueFromRequest(req),
				sentry.WithDescription(functionName),
			)

			if regex.SkipSampling(urlPath) {
				span.Sampled = sentry.SampledFalse
			}
		}
	}

	return span
}

func capturePanic(ctx context.Context, err error) {
	if config.Bean.Sentry.On {
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
