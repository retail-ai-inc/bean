package log

// bench_comparison_test.go
// Side-by-side benchmarks: "Baseline" reproduces the original logic inline;
// "Opt" exercises the current (optimized) code path.
// Run with:
//   go test ./log/... -bench=BenchmarkCmp -benchmem -benchtime=3s

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers shared by comparison benchmarks
// ─────────────────────────────────────────────────────────────────────────────

func makeEntry(withFields bool) Entry {
	e := Entry{
		Timestamp: time.Now(),
		Severity:  Info,
		Level:     "ACCESS",
		Trace:     Trace{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736"},
	}
	if withFields {
		e.Fields = map[string]any{
			"method":        "POST",
			"uri":           "/api/v1/orders",
			"status":        200,
			"latency_ms":    38,
			"user_agent":    "Go-http-client/2.0",
			"request_body":  `plain text body`, // NOT JSON – was passed to Unmarshal anyway in old code
			"response_body": `{"ok":true}`,     // IS JSON
		}
	}
	return e
}

// ─────────────────────────────────────────────────────────────────────────────
// 1.  sink.Write  –  Baseline vs Optimised
//
//  Baseline: map literal alloc every call + json.Marshal + append(b, '\n')
//  Opt:      payloadPool + bufPool + json.NewEncoder  (current sink.Write)
// ─────────────────────────────────────────────────────────────────────────────

// baselineSinkWrite mimics the original sink.Write logic verbatim.
func baselineSinkWrite(writer io.Writer, payloadTrace string, e Entry) error {
	payload := map[string]any{
		"timestamp": e.Timestamp.Format(time.RFC3339Nano),
		"severity":  e.Severity,
		"level":     e.Level,
	}
	for k, v := range e.Fields {
		payload[k] = v
	}
	if e.Trace.TraceID != "" {
		payload[payloadTrace] = e.Trace.TraceID
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = writer.Write(append(b, '\n'))
	return err
}

func BenchmarkCmpSinkWrite_Baseline(b *testing.B) {
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = baselineSinkWrite(io.Discard, "trace", e)
	}
}

func BenchmarkCmpSinkWrite_Opt(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{})
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Write(e)
	}
	b.StopTimer()
	_ = s.Close(context.Background())
}

func BenchmarkCmpSinkWrite_Baseline_Parallel(b *testing.B) {
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = baselineSinkWrite(io.Discard, "trace", e)
		}
	})
}

func BenchmarkCmpSinkWrite_Opt_Parallel(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{})
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = s.Write(e)
		}
	})
	b.StopTimer()
	_ = s.Close(context.Background())
}

// ─────────────────────────────────────────────────────────────────────────────
// 1b.  sink.Write  –  pool+encoder WITHOUT RWMutex (previous atomic.Bool version)
//      vs current RWMutex version. Isolates the cost of the lock.
// ─────────────────────────────────────────────────────────────────────────────

// noLockSinkWrite reproduces the pool+encoder path but without any lock/atomic,
// measuring pure marshal+write cost as the lower bound.
func noLockSinkWrite(writer io.Writer, payloadTrace string, e Entry) error {
	payload := payloadPool.Get().(map[string]any)
	payload["timestamp"] = e.Timestamp.Format(time.RFC3339Nano)
	payload["severity"] = e.Severity
	payload["level"] = e.Level
	for k, v := range e.Fields {
		payload[k] = v
	}
	if e.Trace.TraceID != "" {
		payload[payloadTrace] = e.Trace.TraceID
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
	_, err = writer.Write(buf.Bytes())
	bufPool.Put(buf)
	return err
}

func BenchmarkCmpSinkWrite_NoLock(b *testing.B) {
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = noLockSinkWrite(io.Discard, "trace", e)
	}
}

func BenchmarkCmpSinkWrite_RWMutex(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{})
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Write(e)
	}
	b.StopTimer()
	_ = s.Close(context.Background())
}

func BenchmarkCmpSinkWrite_NoLock_Parallel(b *testing.B) {
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = noLockSinkWrite(io.Discard, "trace", e)
		}
	})
}

func BenchmarkCmpSinkWrite_RWMutex_Parallel(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{})
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = s.Write(e)
		}
	})
	b.StopTimer()
	_ = s.Close(context.Background())
}

// ─────────────────────────────────────────────────────────────────────────────
// 2.  MaskProcessor.Process  –  plain-string fields (NOT JSON)
//
//  Baseline: always calls json.Unmarshal on every string value
//  Opt:      looksLikeJSON prefix check skips Unmarshal for plain strings
// ─────────────────────────────────────────────────────────────────────────────

// baselineMaskValue mimics the original maskValue string case.
type baselineMask struct {
	fields map[string]struct{}
}

func (p *baselineMask) processEntry(entry Entry) Entry {
	if entry.Fields == nil {
		return entry
	}
	masked := p.maskValue(entry.Fields)
	if m, ok := masked.(map[string]any); ok {
		entry.Fields = m
	}
	return entry
}

func (p *baselineMask) maskValue(val any) any {
	switch v := val.(type) {
	case map[string]any:
		for k, vv := range v {
			if _, ok := p.fields[k]; ok {
				v[k] = "****"
			} else {
				v[k] = p.maskValue(vv)
			}
		}
		return v
	case string:
		// OLD: unconditionally try to Unmarshal every string
		var decoded any
		if err := json.Unmarshal([]byte(v), &decoded); err != nil {
			return v
		}
		masked := p.maskValue(decoded)
		b, err := json.Marshal(masked)
		if err != nil {
			return v
		}
		return string(b)
	default:
		return v
	}
}

func BenchmarkCmpMask_PlainString_Baseline(b *testing.B) {
	proc := &baselineMask{fields: map[string]struct{}{"password": {}}}
	e := Entry{Fields: map[string]any{
		"user_agent": "Mozilla/5.0 (Macintosh)",
		"method":     "GET",
		"uri":        "/api/products",
		"status":     "200",
	}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.processEntry(e)
	}
}

func BenchmarkCmpMask_PlainString_Opt(b *testing.B) {
	proc := NewMaskProcessor([]string{"password"})
	e := Entry{Fields: map[string]any{
		"user_agent": "Mozilla/5.0 (Macintosh)",
		"method":     "GET",
		"uri":        "/api/products",
		"status":     "200",
	}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.Process(e)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 3.  MaskProcessor with empty p.fields  –  Baseline vs Opt early-return
// ─────────────────────────────────────────────────────────────────────────────

func BenchmarkCmpMask_EmptyFields_Baseline(b *testing.B) {
	// Old code had no len(p.fields)==0 guard; it would still iterate all Fields.
	proc := &baselineMask{fields: map[string]struct{}{}}
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.processEntry(e)
	}
}

func BenchmarkCmpMask_EmptyFields_Opt(b *testing.B) {
	// New code returns immediately when len(p.fields)==0.
	proc := NewMaskProcessor([]string{})
	e := makeEntry(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.Process(e)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4.  RemoveEscapeProcessor  –  plain-string fields  (Baseline vs Opt)
// ─────────────────────────────────────────────────────────────────────────────

type baselineRemoveEscape struct{}

func (p *baselineRemoveEscape) processEntry(entry Entry) Entry {
	if entry.Fields == nil {
		return entry
	}
	processed := p.removeEscapeValue(entry.Fields)
	if m, ok := processed.(map[string]any); ok {
		entry.Fields = m
	}
	return entry
}

func (p *baselineRemoveEscape) removeEscapeValue(val any) any {
	switch v := val.(type) {
	case string:
		// OLD: always try Unmarshal on every string
		var decoded any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return p.removeEscapeValue(decoded)
		}
		return v
	case map[string]any:
		for k, vv := range v {
			v[k] = p.removeEscapeValue(vv)
		}
		return v
	case []any:
		for i, vv := range v {
			v[i] = p.removeEscapeValue(vv)
		}
		return v
	default:
		return v
	}
}

func BenchmarkCmpRemoveEscape_PlainString_Baseline(b *testing.B) {
	proc := &baselineRemoveEscape{}
	e := Entry{Fields: map[string]any{
		"user_agent": "Mozilla/5.0 (Macintosh)",
		"method":     "GET",
		"uri":        "/api/products",
		"label":      "plain value",
	}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.processEntry(e)
	}
}

func BenchmarkCmpRemoveEscape_PlainString_Opt(b *testing.B) {
	proc := NewRemoveEscapeProcessor()
	e := Entry{Fields: map[string]any{
		"user_agent": "Mozilla/5.0 (Macintosh)",
		"method":     "GET",
		"uri":        "/api/products",
		"label":      "plain value",
	}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.Process(e)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 5.  Full pipeline  –  Baseline vs Opt  (end-to-end realistic workload)
//
//  Simulates a typical access log call: sink.Write + mask + removeEscape,
//  where most string fields are plain text (not JSON).
// ─────────────────────────────────────────────────────────────────────────────

func BenchmarkCmpPipeline_Baseline(b *testing.B) {
	maskProc := &baselineMask{fields: map[string]struct{}{"password": {}}}
	escProc := &baselineRemoveEscape{}

	e := makeEntry(true)
	buf := &bytes.Buffer{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := maskProc.processEntry(e)
		entry = escProc.processEntry(entry)
		_ = baselineSinkWrite(buf, "trace", entry)
		buf.Reset()
	}
}

func BenchmarkCmpPipeline_Opt(b *testing.B) {
	s, _ := NewSink(NopWriteCloser{Writer: io.Discard}, "trace", SinkConfig{})
	pipeline := NewPipeline(s,
		NewMaskProcessor([]string{"password"}),
		NewRemoveEscapeProcessor(),
	)
	e := makeEntry(true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipeline.Process(e)
	}
	b.StopTimer()
	_ = s.Close(context.Background())
}
