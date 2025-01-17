package gopool

import (
	"log"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Register_Pool(t *testing.T) {
	type args struct {
		name string
		pool *ants.Pool
	}
	tests := []struct {
		name    string
		args    args
		call    int
		wantErr bool
	}{
		{
			name: "register",
			args: args{
				name: "test",
				pool: newPool(t),
			},
			call:    1,
			wantErr: false,
		},
		{
			name: "register_nil_pool",
			args: args{
				name: "test",
				pool: nil,
			},
			call:    1,
			wantErr: true,
		},
		{
			name: "register_twice",
			args: args{
				name: "test",
				pool: newPool(t),
			},
			call:    2,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			var err error
			for i := 0; i < tt.call; i++ {
				err = Register(tt.args.name, tt.args.pool)
			}
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Get_Pool(t *testing.T) {
	type args struct {
		name string
		pool *ants.Pool
	}
	tests := []struct {
		name     string
		args     args
		poolName string
		wantErr  bool
	}{
		{
			name: "get",
			args: args{
				name: "test",
				pool: newPool(t),
			},
			poolName: "test",
			wantErr:  false,
		},
		{
			name: "get_not_found",
			args: args{
				name: "test",
				pool: newPool(t),
			},
			poolName: "wrong_name",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			err := Register(tt.args.name, tt.args.pool)
			require.NoError(t, err)

			got, err := GetPool(tt.poolName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func Test_Unregister_All_Pools(t *testing.T) {

	submitTask := func(dur time.Duration) func(*testing.T) error {
		task := func(name string, dur time.Duration) func() {
			return func() {
				log.Printf("[%7s]task start (%s)", name, dur)
				defer log.Printf("[%7s]task end   (%s)", name, dur)
				time.Sleep(dur)
			}
		}

		return func(t *testing.T) error {
			pool := newPool(t)
			err := Register("test", pool)
			if err != nil {
				return err
			}

			err = pool.Submit(task("test", dur))
			if err != nil {
				return err
			}

			defPool := GetDefaultPool()
			err = defPool.Submit(task("default", dur*2))
			if err != nil {
				return err
			}

			return nil
		}
	}

	type args struct {
		timeout time.Duration
	}
	tests := []struct {
		name       string
		args       args
		submitTask func(*testing.T) error
		wantErr    bool
	}{
		{
			name: "unregister_withou_timeout",
			args: args{
				timeout: 0,
			},
			submitTask: nil,
			wantErr:    false,
		},
		{
			name: "unregister_with_timeout_success",
			args: args{
				timeout: 3 * time.Second,
			},
			submitTask: submitTask(1 * time.Second),
			wantErr:    false,
		},
		{
			name: "unregister_with_timeout_fail",
			args: args{
				timeout: 2 * time.Second,
			},
			submitTask: submitTask(1 * time.Second),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			if tt.submitTask != nil {
				err := tt.submitTask(t)
				require.NoError(t, err)
			}

			err := UnregisterAllPoolsTimeout(tt.args.timeout)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Get_Default_Pool(t *testing.T) {
	pool := GetDefaultPool()
	assert.NotNil(t, pool)
}

func newPool(t *testing.T) *ants.Pool {
	t.Helper()

	pool, err := ants.NewPool(0)
	require.NoError(t, err)
	return pool
}

func setup(t *testing.T) {
	t.Helper()

	defaultPool = initDefaultPool()
	pools = make(map[string]*ants.Pool)
}
