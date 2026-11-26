package dbdrivers

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReplicaConn(t *testing.T, primary *miniredis.Miniredis, replica redis.UniversalClient) *RedisDBConn {
	t.Helper()
	p := redis.NewClient(&redis.Options{Addr: primary.Addr()})
	t.Cleanup(func() { _ = p.Close(); _ = replica.Close() })
	return &RedisDBConn{Primary: p, Reads: map[uint64]redis.UniversalClient{0: replica}, readCount: 1}
}

// A miss or a server reply (WRONGTYPE) on the replica must not be retried on the primary.
func TestReadWithFallback_MissAndServerErrorStayOnReplica(t *testing.T) {
	ctx := context.Background()
	primary, replicaSrv := miniredis.RunT(t), miniredis.RunT(t)
	_, err := replicaSrv.Lpush("list", "a")
	require.NoError(t, err)
	conn := newReplicaConn(t, primary, redis.NewClient(&redis.Options{Addr: replicaSrv.Addr()}))

	before := primary.CommandCount()

	got, err := conn.GetString(ctx, "missing")
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = conn.GetString(ctx, "list")
	assert.ErrorContains(t, err, "WRONGTYPE")

	assert.Equal(t, before, primary.CommandCount(), "the primary must not be queried")
}

// A replica that is down: the read goes to the primary, and the replica is skipped for a while.
func TestReadWithFallback_ReplicaDown(t *testing.T) {
	ctx := context.Background()
	primary := miniredis.RunT(t)
	require.NoError(t, primary.Set("k", "from-primary"))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	downAddr := ln.Addr().String()
	require.NoError(t, ln.Close()) // nothing listens there anymore

	var dials atomic.Int32
	replica := redis.NewClient(&redis.Options{
		Addr:       downAddr,
		MaxRetries: -1,
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dials.Add(1)
			return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, network, addr)
		},
	})
	conn := newReplicaConn(t, primary, replica)

	for range 3 {
		got, err := conn.GetString(ctx, "k")
		require.NoError(t, err)
		assert.Equal(t, "from-primary", got)
	}
	assert.Equal(t, int32(1), dials.Load(), "the replica must be skipped after the first connection error")
}

func TestShouldReadFromPrimary(t *testing.T) {
	ctx := context.Background()
	canceled, cancel := context.WithCancel(ctx)
	cancel()

	assert.False(t, shouldReadFromPrimary(ctx, nil))
	assert.False(t, shouldReadFromPrimary(ctx, redis.Nil))
	assert.True(t, shouldReadFromPrimary(ctx, &net.OpError{Op: "dial"}))
	assert.False(t, shouldReadFromPrimary(canceled, &net.OpError{Op: "dial"}))
}
