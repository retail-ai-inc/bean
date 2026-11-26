package dbdrivers

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRedisConns returns a standalone and a cluster connection to the same miniredis server (miniredis answers
// CLUSTER SLOTS as a single node owning every slot).
func newTestRedisConns(t *testing.T) (*miniredis.Miniredis, map[string]*RedisDBConn) {
	t.Helper()
	mr := miniredis.RunT(t)
	standalone := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cluster := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{mr.Addr()}})
	t.Cleanup(func() {
		_ = standalone.Close()
		_ = cluster.Close()
	})
	return mr, map[string]*RedisDBConn{
		"standalone": {Primary: standalone},
		"cluster":    {Primary: cluster, isCluster: true},
	}
}

func TestRedisDBConn_RPush(t *testing.T) {
	ctx := context.Background()
	mr, conns := newTestRedisConns(t)
	for name, conn := range conns {
		t.Run(name, func(t *testing.T) {
			key := "list:" + name
			require.NoError(t, conn.RPush(ctx, key, []string{"a", "b"}))
			require.NoError(t, conn.RPush(ctx, key, []string{"c"}))
			require.NoError(t, conn.RPush(ctx, key, nil))

			got, err := mr.List(key)
			require.NoError(t, err)
			assert.Equal(t, []string{"a", "b", "c"}, got)
		})
	}
}

// MGet returns one entry per key, nil for a missing key, in both modes.
func TestRedisDBConn_MGetMissingKey(t *testing.T) {
	ctx := context.Background()
	mr, conns := newTestRedisConns(t)
	require.NoError(t, mr.Set("a", "1"))
	require.NoError(t, mr.Set("c", "3"))

	for name, conn := range conns {
		t.Run(name, func(t *testing.T) {
			got, err := conn.MGet(ctx, "a", "b", "c")
			require.NoError(t, err)
			assert.Equal(t, []interface{}{"1", nil, "3"}, got)
		})
	}
}

func TestRedisDBConn_KeysAndDelKey(t *testing.T) {
	ctx := context.Background()
	mr, conns := newTestRedisConns(t)

	for name, conn := range conns {
		t.Run(name, func(t *testing.T) {
			mr.FlushAll()
			for _, k := range []string{"user:1", "user:2", "{other}:3", "cart:1"} {
				require.NoError(t, mr.Set(k, "v"))
			}

			keys, err := conn.Keys(ctx, "user:*")
			require.NoError(t, err)
			assert.ElementsMatch(t, []string{"user:1", "user:2"}, keys)

			require.NoError(t, conn.DelKey(ctx, "user:1", "{other}:3", "missing"))
			assert.False(t, mr.Exists("user:1"))
			assert.False(t, mr.Exists("{other}:3"))
			assert.True(t, mr.Exists("user:2"))
		})
	}
}

func TestRedisDBConn_MSetWithTTL_InvalidKey(t *testing.T) {
	_, conns := newTestRedisConns(t)
	assert.NotPanics(t, func() {
		err := conns["cluster"].MSetWithTTL(context.Background(), 0, []interface{}{1, "v"})
		assert.ErrorIs(t, err, ErrRedisInvalidParameter)
	})
}
