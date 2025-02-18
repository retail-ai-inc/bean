package gopool

import (
	"log"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_New_Pool(t *testing.T) {
	type args struct {
		size       *int
		blockAfter *int
	}
	tests := []struct {
		name    string
		args    args
		want    *ants.Pool
		wantErr bool
	}{
		{
			name: "new_pool_success",
			args: args{
				size:       nil,
				blockAfter: nil,
			},
			want:    func() *ants.Pool { p, _ := ants.NewPool(0); return p }(),
			wantErr: false,
		},
		{
			name: "new_pool_with_size_and_block_after",
			args: args{
				size:       func() *int { i := 1; return &i }(),
				blockAfter: func() *int { i := 1; return &i }(),
			},
			want:    func() *ants.Pool { p, _ := ants.NewPool(1, ants.WithMaxBlockingTasks(1)); return p }(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPool(tt.args.size, tt.args.blockAfter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}

			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreUnexported(ants.Pool{})); diff != "" {
				t.Errorf("NewPool() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

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
			name: "register_success",
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
			name: "get_success",
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

	task := func(dur time.Duration) func() {
		return func() {
			log.Printf("task start (%s)", dur)
			defer log.Printf("task end   (%s)", dur)
			time.Sleep(dur)
		}
	}

	submitNTasks := func(task func(), release bool) func(*testing.T) error {
		return func(t *testing.T) error {
			pool := newPool(t)
			err := Register("test", pool)
			if err != nil {
				return err
			}

			n := math.Max(1, float64(rand.Intn(10)))

			defPool := GetDefaultPool()
			for i := 0; i < int(n); i++ {
				err = defPool.Submit(task)
				if err != nil {
					return err
				}
			}

			for i := 0; i < int(n); i++ {
				err = pool.Submit(task)
				if err != nil {
					return err
				}
			}

			if release {
				pool.Release()
				defPool.Release()
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
				timeout: 150 * time.Millisecond,
			},
			submitTask: submitNTasks(task(100*time.Millisecond), false),
			wantErr:    false,
		},
		{
			name: "unregister_with_timeout_fail",
			args: args{
				timeout: 50 * time.Millisecond,
			},
			submitTask: submitNTasks(task(100*time.Millisecond), false),
			wantErr:    true,
		},
		{
			name: "unregister_already_closed_pools",
			args: args{
				timeout: 150 * time.Millisecond,
			},
			submitTask: submitNTasks(task(100*time.Millisecond), true),
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

			// Act
			err := ReleaseAllPools(tt.args.timeout)()

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Empty(t, pools)
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
