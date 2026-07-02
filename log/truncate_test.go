package log

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateBodyFields_DefaultsTo8KB(t *testing.T) {
	payload := map[string]any{"request_body": strings.Repeat("a", DefaultMaxSizeBytes*2)}

	truncateBodyFields(payload, 0)

	requestBody := payload["request_body"].(string)
	assert.Len(t, strings.TrimSuffix(requestBody, truncatedSuffix), DefaultMaxSizeBytes)
	assert.Contains(t, requestBody, "truncated")
}

func TestTruncateBodyFields_OnlyRequestAndResponseBody(t *testing.T) {
	payload := map[string]any{
		"request_body":  strings.Repeat("a", 64),
		"response_body": strings.Repeat("b", 64),
		"message":       strings.Repeat("c", 64),
	}

	truncateBodyFields(payload, 32)

	assert.Len(t, strings.TrimSuffix(payload["request_body"].(string), truncatedSuffix), 32)
	assert.Len(t, strings.TrimSuffix(payload["response_body"].(string), truncatedSuffix), 32)
	assert.Equal(t, strings.Repeat("c", 64), payload["message"])
}

func TestTruncateBodyFields_DoesNotSplitUTF8(t *testing.T) {
	payload := map[string]any{"response_body": "\u3042\u3044\u3046\u3048\u304a\u304b\u304d\u304f\u3051\u3053"}

	truncateBodyFields(payload, 20)
	message := payload["response_body"].(string)

	_, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(message, truncatedSuffix))
	assert.True(t, json.Valid([]byte(`{"response_body":`+mustJSONQuote(t, message)+`}`)))
	assert.LessOrEqual(t, len(strings.TrimSuffix(message, truncatedSuffix)), 20)
}

func TestSinkWriteLimitsRequestAndResponseBodyFields(t *testing.T) {
	buf := &bytes.Buffer{}
	s, err := NewSink(NopWriteCloser{Writer: buf}, "trace", SinkConfig{MaxSizeBytes: 64})
	require.NoError(t, err)

	err = s.Write(Entry{
		Timestamp: time.Unix(0, 0),
		Severity:  Info,
		Level:     "DUMP",
		Fields: map[string]any{
			"request_body":  strings.Repeat("a", 256),
			"response_body": strings.Repeat("b", 256),
			"message":       strings.Repeat("c", 256),
		},
	})
	require.NoError(t, err)
	require.NoError(t, s.Close(context.Background()))

	var got map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got))
	assert.Len(t, strings.TrimSuffix(got["request_body"].(string), truncatedSuffix), 64)
	assert.Len(t, strings.TrimSuffix(got["response_body"].(string), truncatedSuffix), 64)
	assert.Equal(t, strings.Repeat("c", 256), got["message"])
}

func mustJSONQuote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return string(b)
}
