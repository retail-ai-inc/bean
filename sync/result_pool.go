// Package sync provides ways to execute multiple tasks concurrently
// while synchronously managing them at the same time.
package sync

import (
	"context"
	"net/http"
	"net/url"

	"github.com/getsentry/sentry-go"
	"github.com/sourcegraph/conc/panics"
	"github.com/sourcegraph/conc/pool"
)

type ResultPool[T any] struct {
	pool *pool.ResultContextPool[T]
	span *sentry.Span
}

type ResultPoolOption func(*resultPoolOptions)

type resultPoolOptions struct {
	pl                 poolOptions
	collectErroredRlts bool
}

func WithRltMaxGoroutines(max int) ResultPoolOption {
	return func(opts *resultPoolOptions) {
		opts.pl.max = max
	}
}

func WithRltRequest(req *http.Request) ResultPoolOption {
	return func(opts *resultPoolOptions) {
		opts.pl.req = req
	}
}

func WithRltCancelOnFirstErr() ResultPoolOption {
	return func(opts *resultPoolOptions) {
		opts.pl.cancelOnFirstErr = true
	}
}

func WithCollectErroredRlts() ResultPoolOption {
	return func(opts *resultPoolOptions) {
		opts.collectErroredRlts = true
	}
}

func NewResultPool[T any](ctx context.Context, opts ...ResultPoolOption) ResultPool[T] {

	// set default options
	plOpts := &resultPoolOptions{
		pl: poolOptions{
			req: &http.Request{
				Method: "",
				URL:    &url.URL{},
			},
			max:              0,
			cancelOnFirstErr: false,
		},
		collectErroredRlts: false,
	}

	for _, opt := range opts {
		opt(plOpts)
	}

	// set sentry transaction or span
	span := setSpan(ctx, plOpts.pl.req)
	if span != nil {
		ctx = span.Context()
	}

	var pl *pool.ResultContextPool[T]
	if plOpts.pl.cancelOnFirstErr {
		pl = pool.NewWithResults[T]().WithContext(ctx).WithFirstError().WithCancelOnError()
	} else {
		pl = pool.NewWithResults[T]().WithErrors().WithContext(ctx)
	}
	if plOpts.collectErroredRlts {
		pl = pl.WithCollectErrored()
	}
	if plOpts.pl.max > 0 {
		pl.WithMaxGoroutines(plOpts.pl.max)
	}

	return ResultPool[T]{
		pool: pl,
		span: span,
	}
}

func (p *ResultPool[T]) Go(f func(ctx context.Context) (T, error)) {
	p.pool.Go(func(ctx context.Context) (v T, err error) {
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
			v, err = f(ctx)
		})

		if err != nil {
			return v, err
		}

		return v, nil
	})
}

func (p *ResultPool[T]) Wait() ([]T, error) {
	defer func() {
		if p.span != nil {
			p.span.Finish()
		}
	}()

	return p.pool.Wait()
}
