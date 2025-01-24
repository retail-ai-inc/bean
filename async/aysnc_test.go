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
		regPool   func() error
		task      TaskWithCtx
		asyncOpts []AsyncOption
		wantError bool
	}

	tests := []testCase{
		{
			name:      "Successful_task_execution_without_options",
			ctx:       newCtx(),
			task:      task,
			asyncOpts: nil,
			wantError: false,
		},
		{
			name: "Task_execution_with_timeout",
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
			name: "Task_execution_with_a_pool_name",
			ctx:  newCtx(),
			regPool: func() error {
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
			name: "Task_returning_an_error",
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
			name: "Panic_in_task_execution",
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

			if tt.regPool != nil {
				err := tt.regPool()
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
