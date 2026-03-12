package processors

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/retail-ai-inc/bean/v2/logging/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemoveEscapeProcessor(t *testing.T) {
	type args struct{}
	tests := []struct {
		name string
		args args
		want *RemoveEscapeProcessor
	}{
		{
			name: "create_remove_escape_processor",
			args: args{},
			want: &RemoveEscapeProcessor{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewRemoveEscapeProcessor()
			assert.NotNil(t, got)
		})
	}
}

func TestRemoveEscapeProcessor_Process(t *testing.T) {
	type args struct {
		entry types.Entry
	}
	tests := []struct {
		name string
		args args
		want types.Entry
	}{
		{
			name: "process_entry_with_nil_fields",
			args: args{
				entry: types.Entry{
					Timestamp: time.Now(),
					Severity:  types.Info,
					Level:     "info",
					Fields:    nil,
					Trace:     types.Trace{},
				},
			},
			want: types.Entry{
				Timestamp: time.Now(),
				Severity:  types.Info,
				Level:     "info",
				Fields:    nil,
				Trace:     types.Trace{},
			},
		},
		{
			name: "remove_escape_from_simple_string_json",
			args: args{
				entry: types.Entry{
					Fields: map[string]interface{}{
						"escaped_json": `"{\"name\":\"John\",\"age\":30}"`,
						"normal_field": "plain text",
					},
				},
			},
			want: types.Entry{
				Fields: map[string]interface{}{
					"escaped_json": map[string]interface{}{
						"name": "John",
						"age":  float64(30),
					},
					"normal_field": "plain text",
				},
			},
		},
		{
			name: "remove_escape_from_json_raw_message",
			args: args{
				entry: types.Entry{
					Fields: map[string]interface{}{
						"raw_data": json.RawMessage(`"{\"user\":\"test\",\"active\":true}"`),
					},
				},
			},
			want: types.Entry{
				Fields: map[string]interface{}{
					"raw_data": json.RawMessage(`{"user":"test","active":true}`),
				},
			},
		},
		{
			name: "handle_nested_structures_with_escapes",
			args: args{
				entry: types.Entry{
					Fields: map[string]interface{}{
						"config": `"{\"database\":{\"host\":\"localhost\",\"port\":5432}}"`,
						"users": []interface{}{
							`"{\"id\":1,\"name\":\"Alice\"}"`,
							`"{\"id\":2,\"name\":\"Bob\"}"`,
						},
					},
				},
			},
			want: types.Entry{
				Fields: map[string]interface{}{
					"config": map[string]interface{}{
						"database": map[string]interface{}{
							"host": "localhost",
							"port": float64(5432),
						},
					},
					"users": []interface{}{
						map[string]interface{}{
							"id":   float64(1),
							"name": "Alice",
						},
						map[string]interface{}{
							"id":   float64(2),
							"name": "Bob",
						},
					},
				},
			},
		},
		{
			name: "preserve_non_json_strings",
			args: args{
				entry: types.Entry{
					Fields: map[string]interface{}{
						"message":       "Hello World",
						"description":   "This is a test message",
						"special_chars": "Line 1\nLine 2\tTabbed",
					},
				},
			},
			want: types.Entry{
				Fields: map[string]interface{}{
					"message":       "Hello World",
					"description":   "This is a test message",
					"special_chars": "Line 1\nLine 2\tTabbed",
				},
			},
		},
		{
			name: "handle_malformed_json_strings",
			args: args{
				entry: types.Entry{
					Fields: map[string]interface{}{
						"bad_json":     `{"invalid": json`,
						"empty_string": "",
						"whitespace":   "   ",
					},
				},
			},
			want: types.Entry{
				Fields: map[string]interface{}{
					"bad_json":     `{"invalid": json`,
					"empty_string": "",
					"whitespace":   "   ",
				},
			},
		},
		{
			name: "handle_mixed_data_types",
			args: args{
				entry: types.Entry{
					Fields: map[string]interface{}{
						"id":          123,
						"name":        "Test User",
						"active":      true,
						"score":       95.5,
						"nil_val":     nil,
						"json_string": `"{\"valid\":true}"`,
					},
				},
			},
			want: types.Entry{
				Fields: map[string]interface{}{
					"id":      123,
					"name":    "Test User",
					"active":  true,
					"score":   95.5,
					"nil_val": nil,
					"json_string": map[string]interface{}{
						"valid": true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewRemoveEscapeProcessor()
			got := p.Process(tt.args.entry)

			// Special handling for RawMessage comparison
			if rawGot, ok := got.Fields["raw_data"]; ok {
				if rawWant, ok := tt.want.Fields["raw_data"]; ok {
					gotRaw := rawGot.(json.RawMessage)
					wantRaw := rawWant.(json.RawMessage)

					// Parse both to compare content
					var gotParsed, wantParsed interface{}
					require.NoError(t, json.Unmarshal(gotRaw, &gotParsed))
					require.NoError(t, json.Unmarshal(wantRaw, &wantParsed))

					assert.Equal(t, wantParsed, gotParsed)

					// Remove raw_data from comparison since we handled it specially
					delete(got.Fields, "raw_data")
					delete(tt.want.Fields, "raw_data")
				}
			}

			assert.Equal(t, tt.want.Fields, got.Fields)
		})
	}
}

func TestRemoveEscapeProcessor_Process_PreserveMetadata(t *testing.T) {
	now := time.Now()
	trace := types.Trace{
		TraceID: "trace-123",
		SpanID:  "span-456",
	}

	processor := NewRemoveEscapeProcessor()
	entry := types.Entry{
		Timestamp: now,
		Severity:  types.Error,
		Level:     "error",
		Fields: map[string]interface{}{
			"escaped_data": `"{\"message\":\"test\"}"`,
			"normal_data":  "plain text",
		},
		Trace: trace,
	}

	result := processor.Process(entry)

	assert.Equal(t, now, result.Timestamp)
	assert.Equal(t, types.Error, result.Severity)
	assert.Equal(t, "error", result.Level)
	assert.Equal(t, trace, result.Trace)

	// Verify escaped data was processed
	expectedData := map[string]interface{}{"message": "test"}
	assert.Equal(t, expectedData, result.Fields["escaped_data"])
	assert.Equal(t, "plain text", result.Fields["normal_data"])
}

func TestRemoveEscapeProcessor_removeEscapeValue_EdgeCases(t *testing.T) {
	processor := NewRemoveEscapeProcessor()

	t.Run("handle_unsupported_primitive_types", func(t *testing.T) {
		testCases := []interface{}{
			42,            // int
			3.14,          // float64
			true,          // bool
			complex(1, 2), // complex64
			time.Now(),    // time.Time
		}

		for _, testCase := range testCases {
			result := processor.removeEscapeValue(testCase)
			assert.Equal(t, testCase, result)
		}
	})

	t.Run("handle_malformed_json_raw_message", func(t *testing.T) {
		malformedJSON := json.RawMessage("{ invalid json }")
		result := processor.removeEscapeValue(malformedJSON)

		// Should return the original malformed JSON
		assert.Equal(t, malformedJSON, result)
	})

	t.Run("handle_valid_json_without_escapes", func(t *testing.T) {
		validJSON := json.RawMessage(`{"valid": true, "number": 42}`)
		result := processor.removeEscapeValue(validJSON)

		// Should return processed JSON (same content but potentially re-encoded)
		assert.IsType(t, json.RawMessage{}, result)

		var parsedOriginal, parsedResult interface{}
		require.NoError(t, json.Unmarshal(validJSON, &parsedOriginal))
		require.NoError(t, json.Unmarshal(result.(json.RawMessage), &parsedResult))
		assert.Equal(t, parsedOriginal, parsedResult)
	})
}
