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
package sync_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/retail-ai-inc/bean/v2"
	"github.com/retail-ai-inc/bean/v2/sync"
	"github.com/retail-ai-inc/bean/v2/trace"
)

func Test_Pool(t *testing.T) {

	bean.BeanConfig = &bean.Config{
		Sentry: bean.SentryConfig{
			On:               true,
			TracesSampleRate: 1.0,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/hoge", nil)

	taskDur := time.Duration(100) * time.Millisecond
	if taskDur < 0 {
		t.Fatalf("task dur is less than 0")
	}
	task := func(c context.Context) error {
		ctx, finish := trace.StartSpan(c, "task")
		defer finish()

		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		default:
		}
		fmt.Printf("task started for %v\n", taskDur)
		time.Sleep(taskDur)
		fmt.Println("task executed")

		return nil
	}

	taskErr := func(c context.Context) error {
		ctx, finish := trace.StartSpan(c, "task error")
		defer finish()

		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		default:
		}
		fmt.Println("task error started")

		return errors.New("simulated task error")
	}

	taskPanic := func(c context.Context) error {
		ctx, finish := trace.StartSpan(c, "task panic")
		defer finish()

		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		default:
		}
		fmt.Println("task panic started")

		panic("simulated task panic")
	}

	shorterDurThanTask := taskDur / 10
	if shorterDurThanTask < 0 {
		t.Fatalf("shorter dur than task is less than 0")
	}

	ctxTimeout, cancel := context.WithTimeoutCause(
		context.Background(),
		time.Duration(50)*time.Millisecond,
		errors.New("simulated context timeout"),
	)
	defer cancel()

	type args struct {
		ctx   context.Context
		opts  []sync.PoolOption
		tasks []func(context.Context) error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "go tasks with request",
			args: args{
				ctx: context.Background(),
				opts: []sync.PoolOption{
					sync.WithRequest(req),
				},
				tasks: []func(context.Context) error{
					task, task, task, task, task,
				},
			},
			wantErr: false,
		},
		{
			name: "go tasks with max less than tasks",
			args: args{
				ctx: context.Background(),
				opts: []sync.PoolOption{
					sync.WithMaxGoroutines(2),
				},
				tasks: []func(context.Context) error{
					task, task, task, task, task,
				},
			},
			wantErr: false,
		},
		{
			name: "go tasks with errors",
			args: args{
				ctx:  context.Background(),
				opts: nil,
				tasks: []func(context.Context) error{
					task, taskErr, task, task, taskErr,
				},
			},
			wantErr: true,
		},
		{
			name: "go tasks with panic",
			args: args{
				ctx:  context.Background(),
				opts: nil,
				tasks: []func(context.Context) error{
					task, task, taskPanic, task, task,
				},
			},
			wantErr: true,
		},
		{
			name: "go tasks with timeout",
			args: args{
				ctx:  ctxTimeout,
				opts: nil,
				tasks: []func(context.Context) error{
					task, task, task, task, task,
				},
			},
			wantErr: true,
		},
		{
			name: "go tasks with cancel on first error",
			args: args{
				ctx: context.Background(),
				opts: []sync.PoolOption{
					sync.WithCancelOnFirstErr(),
				},
				tasks: []func(context.Context) error{
					task, taskErr, task, task, taskErr,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pool := sync.NewPool(tt.args.ctx, tt.args.opts...)

			for _, task := range tt.args.tasks {
				pool.Go(task)
			}

			something := func(ctx context.Context) error {
				_, finish := trace.StartSpan(ctx, "something")
				defer finish()

				dur := time.Duration(120) * time.Millisecond
				fmt.Printf("something started for %v\n", dur)
				time.Sleep(dur)
				fmt.Println("something executed")

				return nil
			}
			_ = something(tt.args.ctx)

			err := pool.Wait()
			if (err != nil) != tt.wantErr {
				t.Errorf("GoPools() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
