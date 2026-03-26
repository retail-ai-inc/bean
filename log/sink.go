package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var bufPool = sync.Pool{
	New: func() any { return bytes.NewBuffer(make([]byte, 0, 512)) },
}

var payloadPool = sync.Pool{
	New: func() any { return make(map[string]any, 8) },
}

type Sink interface {
	Write(entry Entry) error
}

const defaultAsyncQueueSize = 4096

var ErrSinkClosed = errors.New("log sink is closed")

type SinkConfig struct {
	Async     bool
	QueueSize int
}

// NopWriteCloser wraps an io.Writer with a no-op Close so it satisfies io.WriteCloser.
type NopWriteCloser struct{ io.Writer }

func (NopWriteCloser) Close() error { return nil }

type sink struct {
	out          io.WriteCloser
	payloadTrace string
	async        bool

	queue    chan *bytes.Buffer
	workerWg sync.WaitGroup

	// Write holds RLock; Close holds Lock — guarantees no sender after close(queue).
	mu      sync.RWMutex
	closed  bool
	dropped atomic.Uint64
}

func NewSink(out io.WriteCloser, payloadTrace string, cfg SinkConfig) (*sink, error) {
	gs := &sink{
		out:          out,
		payloadTrace: strings.TrimSpace(payloadTrace),
		async:        cfg.Async,
	}

	if !cfg.Async {
		return gs, nil
	}

	qsize := cfg.QueueSize
	if qsize <= 0 {
		qsize = defaultAsyncQueueSize
	}
	gs.queue = make(chan *bytes.Buffer, qsize)
	gs.workerWg.Add(1)
	go gs.runWriter()

	return gs, nil
}

func (g *sink) Write(e Entry) error {
	g.mu.RLock()
	if g.closed {
		g.mu.RUnlock()
		return ErrSinkClosed
	}
	defer g.mu.RUnlock()

	payload := payloadPool.Get().(map[string]any)
	payload["timestamp"] = e.Timestamp.Format(time.RFC3339Nano)
	payload["severity"] = e.Severity
	payload["level"] = e.Level

	for k, v := range e.Fields {
		payload[k] = v
	}

	if e.Trace.TraceID != "" {
		payload[g.payloadTrace] = e.Trace.TraceID
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	err := json.NewEncoder(buf).Encode(payload)

	clear(payload)
	payloadPool.Put(payload)

	if err != nil {
		bufPool.Put(buf)
		return err
	}

	if !g.async {
		_, err = g.out.Write(buf.Bytes())
		bufPool.Put(buf)
		return err
	}

	select {
	case g.queue <- buf:
		return nil
	default:
		g.dropped.Add(1)
		bufPool.Put(buf)
		return nil
	}
}

func (g *sink) runWriter() {
	defer g.workerWg.Done()
	for buf := range g.queue {
		_, _ = g.out.Write(buf.Bytes())
		bufPool.Put(buf)
	}
}

func (g *sink) Close(ctx context.Context) error {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil
	}
	g.closed = true
	g.mu.Unlock()

	var drainErr error
	if g.async {
		close(g.queue)

		done := make(chan struct{})
		go func() {
			defer close(done)
			g.workerWg.Wait()
		}()

		select {
		case <-done:
		case <-ctx.Done():
			drainErr = ctx.Err()
		}
	}

	if dropped := g.dropped.Load(); dropped > 0 {
		fmt.Fprintf(g.out, "{\"severity\":\"WARNING\",\"message\":\"log entries dropped\",\"dropped_count\":%d}\n", dropped)
	}

	return errors.Join(drainErr, g.out.Close())
}

func (g *sink) DroppedCount() uint64 {
	return g.dropped.Load()
}
