package log

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertFieldsEqual compares got Fields with want Fields. For json.RawMessage it compares
// parsed content; otherwise uses assert.Equal.
func assertFieldsEqual(t *testing.T, want, got map[string]interface{}) {
	t.Helper()
	if want == nil && got == nil {
		return
	}
	require.NotNil(t, got, "got Fields should not be nil when want is not nil")
	require.NotNil(t, want, "want Fields should not be nil when got is not nil")

	for k, wantVal := range want {
		gotVal, ok := got[k]
		require.True(t, ok, "missing key %q in got", k)

		if wantRaw, ok := wantVal.(json.RawMessage); ok {
			gotRaw, ok := gotVal.(json.RawMessage)
			require.True(t, ok, "key %q: want RawMessage but got %T", k, gotVal)
			var wantParsed, gotParsed interface{}
			require.NoError(t, json.Unmarshal(wantRaw, &wantParsed))
			require.NoError(t, json.Unmarshal(gotRaw, &gotParsed))
			assert.Equal(t, wantParsed, gotParsed, "key %q", k)
			continue
		}
		assert.Equal(t, wantVal, gotVal, "key %q", k)
	}
	// Ensure no extra keys in got
	for k := range got {
		_, ok := want[k]
		assert.True(t, ok, "unexpected key %q in got", k)
	}
}

func TestNewRemoveEscapeProcessor(t *testing.T) {
	got := NewRemoveEscapeProcessor()
	assert.NotNil(t, got)
}

func TestRemoveEscapeProcessor_Process(t *testing.T) {
	p := NewRemoveEscapeProcessor()

	tests := []struct {
		name   string
		fields map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name:   "nil fields",
			fields: nil,
			want:   nil,
		},
		{
			name: "simple escaped JSON string",
			fields: map[string]interface{}{
				"escaped_json": `"{\"name\":\"John\",\"age\":30}"`,
				"normal_field": "plain text",
			},
			want: map[string]interface{}{
				"escaped_json": map[string]interface{}{
					"name": "John",
					"age":  float64(30),
				},
				"normal_field": "plain text",
			},
		},
		{
			name: "json.RawMessage with escaped content",
			fields: map[string]interface{}{
				"raw_data": json.RawMessage(`"{\"user\":\"test\",\"active\":true}"`),
			},
			want: map[string]interface{}{
				"raw_data": json.RawMessage(`{"user":"test","active":true}`),
			},
		},
		{
			name: "nested structures with escapes",
			fields: map[string]interface{}{
				"config": `"{\"database\":{\"host\":\"localhost\",\"port\":5432}}"`,
				"users": []interface{}{
					`"{\"id\":1,\"name\":\"Alice\"}"`,
					`"{\"id\":2,\"name\":\"Bob\"}"`,
				},
			},
			want: map[string]interface{}{
				"config": map[string]interface{}{
					"database": map[string]interface{}{
						"host": "localhost",
						"port": float64(5432),
					},
				},
				"users": []interface{}{
					map[string]interface{}{"id": float64(1), "name": "Alice"},
					map[string]interface{}{"id": float64(2), "name": "Bob"},
				},
			},
		},
		{
			name: "preserve non-JSON strings",
			fields: map[string]interface{}{
				"message":       "Hello World",
				"description":   "This is a test message",
				"special_chars": "Line 1\nLine 2\tTabbed",
			},
			want: map[string]interface{}{
				"message":       "Hello World",
				"description":   "This is a test message",
				"special_chars": "Line 1\nLine 2\tTabbed",
			},
		},
		{
			name: "malformed JSON and edge strings",
			fields: map[string]interface{}{
				"bad_json":     `{"invalid": json`,
				"empty_string": "",
				"whitespace":   "   ",
			},
			want: map[string]interface{}{
				"bad_json":     `{"invalid": json`,
				"empty_string": "",
				"whitespace":   "   ",
			},
		},
		{
			name: "mixed types with one escaped JSON",
			fields: map[string]interface{}{
				"id":          123,
				"name":        "Test User",
				"active":      true,
				"score":       95.5,
				"nil_val":     nil,
				"json_string": `"{\"valid\":true}"`,
			},
			want: map[string]interface{}{
				"id":      123,
				"name":    "Test User",
				"active":  true,
				"score":   95.5,
				"nil_val": nil,
				"json_string": map[string]interface{}{"valid": true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := Entry{Fields: tt.fields}
			got := p.Process(entry)
			assertFieldsEqual(t, tt.want, got.Fields)
		})
	}
}

func TestRemoveEscapeProcessor_Process_PreserveMetadata(t *testing.T) {
	now := time.Now()
	trace := Trace{TraceID: "trace-123", SpanID: "span-456"}
	p := NewRemoveEscapeProcessor()

	entry := Entry{
		Timestamp: now,
		Severity:  Error,
		Level:     "error",
		Fields: map[string]interface{}{
			"escaped_data": `"{\"message\":\"test\"}"`,
			"normal_data":  "plain text",
		},
		Trace: trace,
	}

	result := p.Process(entry)

	assert.Equal(t, now, result.Timestamp)
	assert.Equal(t, Error, result.Severity)
	assert.Equal(t, "error", result.Level)
	assert.Equal(t, trace, result.Trace)
	assert.Equal(t, map[string]interface{}{"message": "test"}, result.Fields["escaped_data"])
	assert.Equal(t, "plain text", result.Fields["normal_data"])
}

func TestRemoveEscapeProcessor_removeEscapeValue_EdgeCases(t *testing.T) {
	p := NewRemoveEscapeProcessor()

	t.Run("unsupported primitive types pass through", func(t *testing.T) {
		cases := []interface{}{42, 3.14, true, complex(1, 2), time.Now()}
		for _, c := range cases {
			assert.Equal(t, c, p.removeEscapeValue(c))
		}
	})

	t.Run("malformed json.RawMessage returned unchanged", func(t *testing.T) {
		malformed := json.RawMessage("{ invalid json }")
		assert.Equal(t, malformed, p.removeEscapeValue(malformed))
	})

	t.Run("valid json.RawMessage without escapes preserved", func(t *testing.T) {
		raw := json.RawMessage(`{"valid": true, "number": 42}`)
		got := p.removeEscapeValue(raw)
		assert.IsType(t, json.RawMessage{}, got)

		var wantParsed, gotParsed interface{}
		require.NoError(t, json.Unmarshal(raw, &wantParsed))
		require.NoError(t, json.Unmarshal(got.(json.RawMessage), &gotParsed))
		assert.Equal(t, wantParsed, gotParsed)
	})
}
