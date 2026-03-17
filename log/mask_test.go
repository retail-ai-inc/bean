package log

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMaskProcessor(t *testing.T) {
	type args struct {
		fields []string
	}
	tests := []struct {
		name string
		args args
		want *MaskProcessor
	}{
		{
			name: "create_processor_with_multiple_fields",
			args: args{
				fields: []string{"password", "token", "secret"},
			},
			want: &MaskProcessor{
				fields: map[string]struct{}{
					"password": {},
					"token":    {},
					"secret":   {},
				},
			},
		},
		{
			name: "create_processor_with_single_field",
			args: args{
				fields: []string{"password"},
			},
			want: &MaskProcessor{
				fields: map[string]struct{}{
					"password": {},
				},
			},
		},
		{
			name: "create_processor_with_empty_fields",
			args: args{
				fields: []string{},
			},
			want: &MaskProcessor{
				fields: map[string]struct{}{},
			},
		},
		{
			name: "create_processor_with_nil_fields",
			args: args{
				fields: nil,
			},
			want: &MaskProcessor{
				fields: map[string]struct{}{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMaskProcessor(tt.args.fields)
			assert.NotNil(t, got)
			assert.NotNil(t, got.fields)
			assert.Equal(t, len(tt.want.fields), len(got.fields))

			// Verify all expected fields exist
			for field := range tt.want.fields {
				_, exists := got.fields[field]
				assert.True(t, exists, "field %s should exist", field)
			}
		})
	}
}

func TestMaskProcessor_Process(t *testing.T) {
	type args struct {
		entry Entry
	}
	tests := []struct {
		name             string
		fieldsToMask     []string
		args             args
		want             Entry
		wantFieldsMasked bool
	}{
		{
			name:         "process_entry_with_nil_fields",
			fieldsToMask: []string{"password"},
			args: args{
				entry: Entry{
					Timestamp: time.Now(),
					Severity:  Info,
					Level:     "info",
					Fields:    nil,
					Trace:     Trace{},
				},
			},
			want: Entry{
				Timestamp: time.Now(),
				Severity:  Info,
				Level:     "info",
				Fields:    nil,
				Trace:     Trace{},
			},
			wantFieldsMasked: false,
		},
		{
			name:         "mask_simple_string_fields",
			fieldsToMask: []string{"password", "token"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						"username": "john_doe",
						"password": "secret123",
						"email":    "john@example.com",
						"token":    "abc123xyz",
					},
				},
			},
			want: Entry{
				Fields: map[string]interface{}{
					"username": "john_doe",
					"password": "****",
					"email":    "john@example.com",
					"token":    "****",
				},
			},
			wantFieldsMasked: true,
		},
		{
			name:         "mask_nested_map_fields",
			fieldsToMask: []string{"ssn", "credit_card"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						"user": map[string]interface{}{
							"name":  "John Doe",
							"ssn":   "123-45-6789",
							"phone": "555-1234",
						},
						"payment": map[string]interface{}{
							"method":      "credit",
							"credit_card": "4111-1111-1111-1111",
							"amount":      100.50,
						},
					},
				},
			},
			want: Entry{
				Fields: map[string]interface{}{
					"user": map[string]interface{}{
						"name":  "John Doe",
						"ssn":   "****",
						"phone": "555-1234",
					},
					"payment": map[string]interface{}{
						"method":      "credit",
						"credit_card": "****",
						"amount":      100.50,
					},
				},
			},
			wantFieldsMasked: true,
		},
		{
			name:         "mask_array_elements",
			fieldsToMask: []string{"password"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						"users": []interface{}{
							map[string]interface{}{
								"username": "user1",
								"password": "pass1",
							},
							map[string]interface{}{
								"username": "user2",
								"password": "pass2",
							},
						},
					},
				},
			},
			want: Entry{
				Fields: map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{
							"username": "user1",
							"password": "****",
						},
						map[string]interface{}{
							"username": "user2",
							"password": "****",
						},
					},
				},
			},
			wantFieldsMasked: true,
		},
		{
			name:         "mask_json_raw_message_fields",
			fieldsToMask: []string{"secret_key"},
			args: args{
				entry: func() Entry {
					nestedJSON := `{
						"api_key": "public_key_123",
						"secret_key": "private_secret_456",
						"config": {
							"debug": true,
							"secret_key": "nested_secret_789"
						}
					}`
					return Entry{
						Fields: map[string]interface{}{
							"credentials": json.RawMessage(nestedJSON),
						},
					}
				}(),
			},
			wantFieldsMasked: true,
		},
		{
			name:         "mask_json_byte_slice_value",
			fieldsToMask: []string{"secret_key"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						"credentials": []byte(`{"secret_key":"private_secret_456","config":{"secret_key":"nested_secret_789"}}`),
					},
				},
			},
			wantFieldsMasked: true,
		},
		{
			name:         "preserve_plain_string_value",
			fieldsToMask: []string{"password"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						"message": "hello world",
					},
				},
			},
			want: Entry{
				Fields: map[string]interface{}{
					"message": "hello world",
				},
			},
			wantFieldsMasked: false,
		},
		{
			name:         "mask_json_string_value",
			fieldsToMask: []string{"secret_key"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						// A JSON object stored as a string (common in logs).
						"credentials": `{"api_key":"public_key_123","secret_key":"private_secret_456","config":{"secret_key":"nested_secret_789"}}`,
					},
				},
			},
			want: Entry{
				Fields: map[string]interface{}{
					"credentials": `{"api_key":"public_key_123","config":{"secret_key":"****"},"secret_key":"****"}`,
				},
			},
			wantFieldsMasked: true,
		},
		{
			name:         "handle_mixed_data_types",
			fieldsToMask: []string{"password", "token"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						"id":       123,
						"name":     "Test User",
						"password": "secret",
						"active":   true,
						"token":    "abc123",
						"score":    95.5,
						"nil_val":  nil,
					},
				},
			},
			want: Entry{
				Fields: map[string]interface{}{
					"id":       123,
					"name":     "Test User",
					"password": "****",
					"active":   true,
					"token":    "****",
					"score":    95.5,
					"nil_val":  nil,
				},
			},
			wantFieldsMasked: true,
		},
		{
			name:         "preserve_non_matching_fields",
			fieldsToMask: []string{"password", "token"},
			args: args{
				entry: Entry{
					Fields: map[string]interface{}{
						"username":    "john_doe",
						"email":       "john@example.com",
						"description": "This is a test user",
						"count":       42,
					},
				},
			},
			want: Entry{
				Fields: map[string]interface{}{
					"username":    "john_doe",
					"email":       "john@example.com",
					"description": "This is a test user",
					"count":       42,
				},
			},
			wantFieldsMasked: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewMaskProcessor(tt.fieldsToMask)
			got := p.Process(tt.args.entry)

			if tt.wantFieldsMasked {
				// For cases where we expect masking, verify the masked values
				if tt.want.Fields != nil {
					assert.Equal(t, tt.want.Fields, got.Fields)
				} else {
					// For RawMessage/[]byte(JSON) cases, we need special handling and MUST assert type.
					raw, ok := got.Fields["credentials"].(json.RawMessage)
					require.True(t, ok, "credentials should be json.RawMessage after masking")

					var parsed map[string]interface{}
					err := json.Unmarshal(raw, &parsed)
					require.NoError(t, err)
					assert.Equal(t, "****", parsed["secret_key"])

					if config, ok := parsed["config"].(map[string]interface{}); ok {
						assert.Equal(t, "****", config["secret_key"])
					}
				}
			} else {
				// For cases where no masking should occur
				assert.Equal(t, tt.args.entry, got)
			}
		})
	}
}

func TestMaskProcessor_Process_PreserveMetadata(t *testing.T) {
	now := time.Now()
	trace := Trace{
		TraceID: "trace-123",
		SpanID:  "span-456",
	}

	processor := NewMaskProcessor([]string{"password"})
	entry := Entry{
		Timestamp: now,
		Severity:  Error,
		Level:     "error",
		Fields: map[string]interface{}{
			"password": "secret",
			"message":  "test message",
		},
		Trace: trace,
	}

	result := processor.Process(entry)

	assert.Equal(t, now, result.Timestamp)
	assert.Equal(t, Error, result.Severity)
	assert.Equal(t, "error", result.Level)
	assert.Equal(t, trace, result.Trace)
	assert.Equal(t, "****", result.Fields["password"])
	assert.Equal(t, "test message", result.Fields["message"])
}

func TestMaskProcessor_maskValue_EdgeCases(t *testing.T) {
	processor := NewMaskProcessor([]string{"test"})

	t.Run("handle_unsupported_primitive_types", func(t *testing.T) {
		testCases := []interface{}{
			42,              // int
			3.14,            // float64
			true,            // bool
			"simple string", // string
		}

		for _, testCase := range testCases {
			entry := Entry{
				Fields: map[string]interface{}{
					"value": testCase,
				},
			}

			result := processor.Process(entry)
			assert.Equal(t, testCase, result.Fields["value"])
		}
	})

	t.Run("handle_malformed_json_raw_message", func(t *testing.T) {
		malformedJSON := json.RawMessage("{ invalid json }")
		entry := Entry{
			Fields: map[string]interface{}{
				"data": malformedJSON,
			},
		}

		result := processor.Process(entry)

		// Should return the original malformed JSON
		assert.Equal(t, malformedJSON, result.Fields["data"])
	})
}
