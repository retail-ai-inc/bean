package dbdrivers

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisDBConn_SetNX(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	conn := &RedisDBConn{
		Primary: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
	}
	ctx := context.Background()
	const key = "idempotency:test-key"
	const value = "first-value"
	ttl := time.Hour

	ok, err := conn.SetNX(ctx, key, value, ttl)
	require.NoError(t, err)
	assert.True(t, ok, "first SetNX should acquire the key")

	got, err := conn.GetString(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	mrTTL := mr.TTL(key)
	assert.Greater(t, mrTTL, time.Duration(0))
	assert.LessOrEqual(t, mrTTL, ttl)

	ok, err = conn.SetNX(ctx, key, "second-value", ttl)
	require.NoError(t, err)
	assert.False(t, ok, "second SetNX should fail when key exists")

	got, err = conn.GetString(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got, "existing value must not be overwritten")
}

func TestRedisDBConn_SetNX_afterExpiry(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	conn := &RedisDBConn{
		Primary: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
	}
	ctx := context.Background()
	const key = "idempotency:expire-key"

	ok, err := conn.SetNX(ctx, key, "v1", time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	mr.FastForward(2 * time.Second)

	ok, err = conn.SetNX(ctx, key, "v2", time.Second)
	require.NoError(t, err)
	assert.True(t, ok, "SetNX should succeed after key expires")

	got, err := conn.GetString(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "v2", got)
}


func Test_connectRedisDB(t *testing.T) {
	type args struct {
		password           string
		host               string
		port               string
		dbName             int
		maxretries         int
		poolsize           int
		minIdleConnections int
		dialTimeout        time.Duration
		readTimeout        time.Duration
		writeTimeout       time.Duration
		poolTimeout        time.Duration
		readOnly           bool
		ssl                SSLConfig
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "success",
			args: args{
				password:           "AqNUe43qWL",
				host:               "34.84.56.20,35.200.65.32,35.243.97.67,35.243.119.218,35.187.198.109,104.198.85.119",
				port:               "6379",
				dbName:             0,
				maxretries:         -2,
				poolsize:           0,
				minIdleConnections: 0,
				dialTimeout:        0,
				readTimeout:        0,
				writeTimeout:       0,
				poolTimeout:        0,
				readOnly:           false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := connectRedisDB(tt.args.password, tt.args.host, tt.args.port, tt.args.dbName, tt.args.maxretries, tt.args.poolsize, tt.args.minIdleConnections, tt.args.dialTimeout, tt.args.readTimeout, tt.args.writeTimeout, tt.args.poolTimeout, tt.args.readOnly, tt.args.ssl)
			assert.NoError(t, err)
		})
	}
}
