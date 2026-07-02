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

func TestTruncatePayloadToSize_DefaultsTo4KB(t *testing.T) {
	payload := map[string]any{"message": strings.Repeat("a", DefaultMaxSizeBytes*2)}

	truncatePayloadToSize(payload, 0)

	assert.LessOrEqual(t, encodedPayloadSize(payload), DefaultMaxSizeBytes)
}

func TestTruncatePayloadToSize_TruncatesNestedValues(t *testing.T) {
	payload := map[string]any{
		"short": "ok",
		"nested": map[string]any{
			"long": strings.Repeat("b", 512),
		},
		"list": []any{strings.Repeat("c", 512)},
	}

	truncatePayloadToSize(payload, 256)

	assert.Equal(t, "ok", payload["short"])
	assert.LessOrEqual(t, encodedPayloadSize(payload), 256)
	assert.Contains(t, payload["nested"].(map[string]any)["long"].(string), "truncated")
}

func TestTruncatePayloadToSize_DoesNotSplitUTF8(t *testing.T) {
	payload := map[string]any{"message": "\u3042\u3044\u3046\u3048\u304a\u304b\u304d\u304f\u3051\u3053"}

	truncatePayloadToSize(payload, 40)
	message := payload["message"].(string)

	_, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(message, truncatedSuffix))
	assert.True(t, json.Valid([]byte(`{"message":`+mustJSONQuote(t, message)+`}`)))
}

func TestSinkWriteLimitsEncodedEntrySize(t *testing.T) {
	buf := &bytes.Buffer{}
	s, err := NewSink(NopWriteCloser{Writer: buf}, "trace", SinkConfig{MaxSizeBytes: 512})
	require.NoError(t, err)

	err = s.Write(Entry{
		Timestamp: time.Unix(0, 0),
		Severity:  Info,
		Level:     "DUMP",
		Fields: map[string]any{
			"request_body":  strings.Repeat("a", 2048),
			"response_body": strings.Repeat("b", 2048),
		},
	})
	require.NoError(t, err)
	require.NoError(t, s.Close(context.Background()))

	assert.LessOrEqual(t, buf.Len(), 512)
	assert.Contains(t, buf.String(), "truncated")
	assert.True(t, json.Valid(bytes.TrimSpace(buf.Bytes())))
}

func mustJSONQuote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return string(b)
}
