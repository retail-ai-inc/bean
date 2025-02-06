package async

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/retail-ai-inc/bean/v2/config"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/retail-ai-inc/bean/v2/internal/gopool"
	"github.com/retail-ai-inc/bean/v2/log"
	"github.com/retail-ai-inc/bean/v2/trace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Execute_Context(t *testing.T) {
	config.Bean = &config.Config{Sentry: config.Sentry{
		On:               false,
		TracesSampleRate: 0,
	},
	}
	_ = log.New()

	task := func(ctx context.Context) error {
		_, finish := trace.StartSpan(ctx, "test")
		defer finish()
		return nil
	}

	newCtx := func() context.Context {
		return bctx.SetRequestID(context.Background(), uuid.NewString())
	}

	type testCase struct {
		name      string
		ctx       context.Context
		setupPool func() error
		task      TaskWithCtx
		asyncOpts []AsyncOption
		wantError bool
	}

	tests := []testCase{
		{
			name:      "task_execution_without_options_success",
			ctx:       newCtx(),
			task:      task,
			asyncOpts: nil,
			wantError: false,
		},
		{
			name: "task_execution_with_timeout",
			ctx:  newCtx(),
			task: func(ctx context.Context) error {
				_, finish := trace.StartSpan(ctx, "test")
				defer finish()

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
					return nil
				}
			},
			asyncOpts: []AsyncOption{WithTimeout(50 * time.Millisecond)},
			wantError: false,
		},
		{
			name: "task_execution_with_a_pool_name",
			ctx:  newCtx(),
			setupPool: func() error {
				pool, err := gopool.NewPool(nil, nil)
				if err != nil {
					return err
				}
				return gopool.Register("testPool", pool)
			},
			task:      task,
			asyncOpts: []AsyncOption{WithPoolName("testPool")},
			wantError: false,
		},
		{
			name: "task_execution_with_non_existent_pool_name",
			ctx:  newCtx(),
			task: task,
			asyncOpts: []AsyncOption{
				WithPoolName("nonExistentPool"),
			},
			wantError: false,
		},
		{
			name: "task_submission_error_with_pool_name",
			ctx:  newCtx(),
			setupPool: func() error {
				size := 1
				blockAfter := 1
				pool, err := gopool.NewPool(&size, &blockAfter)
				if err != nil {
					return err
				}
				err = gopool.Register("testPool2", pool)
				if err != nil {
					return err
				}

				testPool2, err := gopool.GetPool("testPool2")
				if err != nil {
					return err
				}

				// capacity is full after this task submission
				err = testPool2.Submit(func() {
					time.Sleep(60 * time.Second)
				})
				if err != nil {
					return err
				}

				go func() {
					// max blocking tasks limit is reached after this task submission
					err := testPool2.Submit(func() {
						time.Sleep(60 * time.Second)
					})
					require.Error(t, err)
				}()
				time.Sleep(100 * time.Millisecond) // wait for the goroutine to submit the task

				return nil
			},
			asyncOpts: []AsyncOption{WithPoolName("testPool2")},
			task:      task,
			wantError: true,
		},
		{
			name: "task_returning_an_error",
			ctx:  newCtx(),
			task: func(ctx context.Context) error {
				_, finish := trace.StartSpan(ctx, "test")
				defer finish()

				return fmt.Errorf("test task error")
			},
			asyncOpts: nil,
			wantError: false,
		},
		{
			name: "panic_in_task_execution",
			ctx:  newCtx(),
			task: func(ctx context.Context) error {
				_, finish := trace.StartSpan(ctx, "test")
				defer finish()

				panic("simulated panic")
			},
			asyncOpts: nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.setupPool != nil {
				err := tt.setupPool()
				require.NoError(t, err)
			}

			// Act
			err := ExecuteContext(tt.ctx, tt.task, tt.asyncOpts...)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
