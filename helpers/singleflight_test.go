package helpers

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
)

var timeoutCtx, _ = context.WithTimeout(context.Background(), time.Second)

func TestSingleDo(t *testing.T) {
	type args[T any] struct {
		ctx   context.Context
		key   string
		call  func() (T, error)
		retry uint
		ttl   []time.Duration
	}

	tests := []struct {
		name     string
		args     args[string]
		previous *args[int]
		wantData any
		wantErr  bool
	}{
		{
			name: "success",
			args: args[string]{
				ctx: context.Background(),
				key: "test1",
				call: func() (string, error) {
					return "data", nil
				},
				retry: 2,
				ttl:   nil,
			},
			wantData: "data",
			wantErr:  false,
		},
		{
			name: "failed with ttl",
			args: args[string]{
				ctx: context.Background(),
				key: "test2",
				call: func() (string, error) {
					time.Sleep(time.Second * 2)
					return "", errors.New("some error")
				},
				retry: 0,
				ttl:   []time.Duration{time.Second},
			},
			wantData: "",
			wantErr:  true,
		},
		{
			name: "normal failed",
			args: args[string]{
				ctx: context.Background(),
				key: "test3",
				call: func() (string, error) {
					return "", errors.New("some error")
				},
				retry: 0,
				ttl:   nil,
			},
			wantData: "",
			wantErr:  true,
		},
		{
			name: "error timeout",
			args: args[string]{
				ctx: timeoutCtx,
				key: "test4",
				call: func() (string, error) {
					time.Sleep(time.Millisecond * 500)
					return "", errors.New("some error")
				},
				retry: 2,
				ttl:   nil,
			},
			wantData: "",
			wantErr:  true,
		},
		{
			name: "context deadline exceeded",
			args: args[string]{
				ctx: timeoutCtx,
				key: "test5",
				call: func() (string, error) {
					time.Sleep(time.Millisecond * 500)
					return "", fmt.Errorf("%w", context.DeadlineExceeded)
				},
				retry: 2,
				ttl:   nil,
			},
			wantData: "",
			wantErr:  true,
		},
		{
			name: "type error",
			args: args[string]{
				ctx: timeoutCtx,
				key: "test6",
				call: func() (string, error) {
					return "123", nil
				},
				retry: 0,
				ttl:   nil,
			},
			previous: &args[int]{
				ctx: timeoutCtx,
				key: "test6",
				call: func() (int, error) {
					time.Sleep(time.Millisecond * 500)
					return 123, nil
				},
				retry: 0,
				ttl:   nil,
			},
			wantData: "",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(tt.args.ctx)
			defer cancel()
			if tt.previous != nil {
				go func() {
					_, _ = SingleDo(ctx, tt.previous.key, tt.previous.call, tt.previous.retry, tt.previous.ttl...)
				}()
				time.Sleep(time.Millisecond * 100)
			}
			gotData, err := SingleDo(ctx, tt.args.key, tt.args.call, tt.args.retry, tt.args.ttl...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SingleDo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotData, tt.wantData) {
				t.Errorf("SingleDo() gotData = %v, want %v", gotData, tt.wantData)
			}
		})
	}
}
