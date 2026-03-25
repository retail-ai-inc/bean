package log

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSinkAsyncWriteAndCloseFlush(t *testing.T) {
	var buf bytes.Buffer
	s, err := NewSink(NopWriteCloser{Writer: &buf}, "trace", SinkConfig{Async: true, QueueSize: 8})
	require.NoError(t, err)

	err = s.Write(Entry{
		Timestamp: time.Now(),
		Severity:  Info,
		Level:     "ACCESS",
		Fields: map[string]any{
			"id": "req-1",
		},
	})
	require.NoError(t, err)

	err = s.Close(context.Background())
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "\"level\":\"ACCESS\"")
	assert.Contains(t, out, "\"id\":\"req-1\"")
}

func TestSinkDropWhenQueueFull(t *testing.T) {
	// no-op writer makes the worker run, but a small queue still allows us to
	// observe drop-new behavior under burst writes.
	var buf bytes.Buffer
	s, err := NewSink(NopWriteCloser{Writer: &buf}, "trace", SinkConfig{Async: true, QueueSize: 1})
	require.NoError(t, err)

	for i := 0; i < 1000; i++ {
		_ = s.Write(Entry{
			Timestamp: time.Now(),
			Severity:  Info,
			Level:     "BURST",
			Fields: map[string]any{
				"n": i,
			},
		})
	}

	require.NoError(t, s.Close(context.Background()))
	assert.Greater(t, s.DroppedCount(), uint64(0))
}

func TestSinkWriteAfterClose(t *testing.T) {
	var buf bytes.Buffer
	s, err := NewSink(NopWriteCloser{Writer: &buf}, "trace", SinkConfig{Async: true, QueueSize: 2})
	require.NoError(t, err)
	require.NoError(t, s.Close(context.Background()))

	err = s.Write(Entry{
		Timestamp: time.Now(),
		Severity:  Info,
		Level:     "AFTER_CLOSE",
	})
	require.ErrorIs(t, err, ErrSinkClosed)
}

// TestSinkConcurrentWriteAndClose exercises the exact race window that used to
// cause "send on closed channel" panics: many goroutines writing concurrently
// while Close is called from a separate goroutine.
// Run with: go test -race -run TestSinkConcurrentWriteAndClose ./log/...
func TestSinkConcurrentWriteAndClose(t *testing.T) {
	const workers = 64
	const writesPerWorker = 200

	for trial := 0; trial < 5; trial++ {
		s, err := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{Async: true, QueueSize: 16})
		require.NoError(t, err)

		start := make(chan struct{})
		done := make(chan struct{})

		// Start writers.
		for i := 0; i < workers; i++ {
			go func() {
				<-start
				e := Entry{
					Timestamp: time.Now(),
					Severity:  Info,
					Level:     "CONCURRENT",
					Fields:    map[string]any{"x": 1},
				}
				for j := 0; j < writesPerWorker; j++ {
					_ = s.Write(e) // must NOT panic
				}
			}()
		}

		// Trigger close while writes are in flight.
		go func() {
			<-start
			_ = s.Close(context.Background())
			close(done)
		}()

		close(start) // unleash writers and closer simultaneously
		<-done
	}
}

func TestSinkSyncWritesImmediately(t *testing.T) {
	var buf bytes.Buffer
	s, err := NewSink(NopWriteCloser{Writer: &buf}, "trace", SinkConfig{})
	require.NoError(t, err)

	err = s.Write(Entry{
		Timestamp: time.Now(),
		Severity:  Info,
		Level:     "SYNC",
		Fields:    map[string]any{"k": "v"},
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "\"level\":\"SYNC\"")
	assert.Equal(t, uint64(0), s.DroppedCount())
	require.NoError(t, s.Close(context.Background()))
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func benchEntry() Entry {
	return Entry{
		Timestamp: time.Now(),
		Severity:  Info,
		Level:     "ACCESS",
		Fields: map[string]any{
			"id":         "req-abc-123",
			"method":     "POST",
			"uri":        "/api/v1/users",
			"status":     200,
			"latency_ms": 42,
			"user_agent": "Go-http-client/1.1",
		},
		Trace: Trace{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736"},
	}
}

func BenchmarkSinkSync(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{})
	entry := benchEntry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Write(entry)
	}
	b.StopTimer()
	_ = s.Close(context.Background())
}

func BenchmarkSinkAsync(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{Async: true, QueueSize: 4096})
	entry := benchEntry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Write(entry)
	}
	b.StopTimer()
	_ = s.Close(context.Background())
}

func BenchmarkSinkSyncParallel(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{})
	entry := benchEntry()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = s.Write(entry)
		}
	})
	b.StopTimer()
	_ = s.Close(context.Background())
}

func BenchmarkSinkAsyncParallel(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{Async: true, QueueSize: 4096})
	entry := benchEntry()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = s.Write(entry)
		}
	})
	b.StopTimer()
	_ = s.Close(context.Background())
}
