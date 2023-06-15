package helpers

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
)

var timeoutCtx, _ = context.WithTimeout(context.Background(), time.Second)

func TestSingleDoChan(t *testing.T) {
	type T any
	type args struct {
		ctx   context.Context
		key   string
		call  func() (any, error)
		retry int
		ttl   []time.Duration
	}

	tests := []struct {
		name     string
		args     args
		wantData T
		wantErr  bool
	}{
		{
			name: "success",
			args: args{
				ctx: context.Background(),
				key: "test1",
				call: func() (any, error) {
					return "data", nil
				},
				retry: 2,
				ttl:   nil,
			},
			wantData: "data",
			wantErr:  false,
		},
		{
			name: "retry<0",
			args: args{
				ctx: context.Background(),
				key: "test1",
				call: func() (any, error) {
					var data = "data"
					return &data, nil
				},
				retry: -1,
				ttl:   nil,
			},
			wantData: nil,
			wantErr:  false,
		},
		{
			name: "retry<0 with error",
			args: args{
				ctx: context.Background(),
				key: "test1",
				call: func() (any, error) {
					var data = "data"
					return &data, errors.New("some error")
				},
				retry: -1,
				ttl:   nil,
			},
			wantData: nil,
			wantErr:  false,
		},
		{
			name: "failed with ttl",
			args: args{
				ctx: context.Background(),
				key: "test2",
				call: func() (any, error) {
					time.Sleep(time.Second * 2)
					return nil, errors.New("some error")
				},
				retry: 2,
				ttl:   []time.Duration{time.Second},
			},
			wantData: nil,
			wantErr:  true,
		},
		{
			name: "normal failed",
			args: args{
				ctx: context.Background(),
				key: "test2",
				call: func() (any, error) {
					return 123, errors.New("some error")
				},
				retry: 2,
				ttl:   nil,
			},
			wantData: nil,
			wantErr:  true,
		},
		{
			name: "panic recover",
			args: args{
				ctx: context.Background(),
				key: "test3",
				call: func() (any, error) {
					panic("cause panic here")
				},
				retry: 2,
				ttl:   nil,
			},
			wantData: nil,
			wantErr:  true,
		},
		{
			name: "timeout failed",
			args: args{
				ctx: timeoutCtx,
				key: "test4",
				call: func() (any, error) {
					time.Sleep(time.Second * 2)
					return "data", nil
				},
				retry: 2,
				ttl:   nil,
			},
			wantData: nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(tt.args.ctx)
			defer cancel()
			gotData, err := SingleDoChan[T](ctx, tt.args.key, func() (T, error) {
				res, err := tt.args.call()
				if err != nil {
					return nil, err
				}
				return res, nil
			}, tt.args.retry, tt.args.ttl...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SingleDoChan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotData, tt.wantData) {
				t.Errorf("SingleDoChan() gotData = %v, want %v", gotData, tt.wantData)
			}
		})
	}
}
