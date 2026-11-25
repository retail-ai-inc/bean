package dbdrivers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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

// Test_connectRedisDB connects to a real Redis cluster, given by BEAN_TEST_REDIS_CLUSTER_HOSTS (comma separated
// hosts) and BEAN_TEST_REDIS_CLUSTER_PASSWORD; it is skipped when they are not set.
func Test_connectRedisDB(t *testing.T) {
	hosts := os.Getenv("BEAN_TEST_REDIS_CLUSTER_HOSTS")
	if hosts == "" {
		t.Skip("BEAN_TEST_REDIS_CLUSTER_HOSTS is not set")
	}

	_, _, closeDB, err := connectRedisDB(os.Getenv("BEAN_TEST_REDIS_CLUSTER_PASSWORD"), hosts, "6379", 0,
		-2, 0, 0, 0, 0, 0, 0, false, SSLConfig{})
	require.NoError(t, err)
	assert.NoError(t, closeDB())
}
