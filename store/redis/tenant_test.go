package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/v3/internal/dbdrivers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MSetJSON must store JSON (readable by GetJSON / MGetJSON), like SetJSON.
func TestTenantCache_MSetJSONAndMGetJSON(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	cache := NewTenantCache(map[uint64]*dbdrivers.RedisDBConn{1: {Primary: client}}, "app")
	ctx := context.Background()

	err := cache.MSetJSON(ctx, 1, []string{"a", "b"}, []interface{}{item{1, "x"}, item{2, "y"}}, time.Minute)
	require.NoError(t, err)

	raw, err := mr.Get("app_a") // prefix + default separator "_"
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":1,"name":"x"}`, raw)

	var one item
	found, err := cache.GetJSON(ctx, 1, "b", &one)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, item{2, "y"}, one)

	var many []item
	require.NoError(t, cache.MGetJSON(ctx, 1, &many, "a", "missing", "b"))
	assert.Equal(t, []item{{1, "x"}, {2, "y"}}, many)
}

func TestTenantCache_HGetsKeysAndUnknownTenant(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewTenantCache(map[uint64]*dbdrivers.RedisDBConn{1: {Primary: client}}, "app")
	ctx := context.Background()

	require.NoError(t, cache.HSet(ctx, 1, "h1", "f", "v1"))
	got, err := cache.HGets(ctx, 1, map[string]string{"h1": "f", "h2": "f"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"h1": "v1", "h2": ""}, got, "keyed by the caller's keys, not the prefixed ones")

	_, err = cache.GetString(ctx, 2, "k")
	assert.ErrorIs(t, err, ErrTenantNotFound)
}
