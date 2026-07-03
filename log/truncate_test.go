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

func TestTruncateBodyProcessor_DefaultsTo8KB(t *testing.T) {
	p := NewTruncateBodyProcessor(0)
	entry := Entry{Fields: map[string]any{"request_body": strings.Repeat("a", DefaultBodyLimit*2)}}

	got := p.Process(entry)

	requestBody := got.Fields["request_body"].(string)
	assert.Len(t, strings.TrimSuffix(requestBody, truncatedSuffix), DefaultBodyLimit)
	assert.Contains(t, requestBody, "truncated")
}

func TestTruncateBodyProcessor_OnlyRequestAndResponseBody(t *testing.T) {
	p := NewTruncateBodyProcessor(32)
	entry := Entry{Fields: map[string]any{
		"request_body":  strings.Repeat("a", 64),
		"response_body": strings.Repeat("b", 64),
		"message":       strings.Repeat("c", 64),
	}}

	got := p.Process(entry)

	assert.Len(t, strings.TrimSuffix(got.Fields["request_body"].(string), truncatedSuffix), 32)
	assert.Len(t, strings.TrimSuffix(got.Fields["response_body"].(string), truncatedSuffix), 32)
	assert.Equal(t, strings.Repeat("c", 64), got.Fields["message"])
}

func TestTruncateBodyProcessor_DoesNotSplitUTF8(t *testing.T) {
	p := NewTruncateBodyProcessor(20)
	entry := Entry{Fields: map[string]any{"response_body": "\u3042\u3044\u3046\u3048\u304a\u304b\u304d\u304f\u3051\u3053"}}

	got := p.Process(entry)
	message := got.Fields["response_body"].(string)

	_, err := json.Marshal(got.Fields)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(message, truncatedSuffix))
	assert.True(t, json.Valid([]byte(`{"response_body":`+mustJSONQuote(t, message)+`}`)))
	assert.LessOrEqual(t, len(strings.TrimSuffix(message, truncatedSuffix)), 20)
}

func TestTruncateBodyProcessor_MarshalsStructuredBody(t *testing.T) {
	p := NewTruncateBodyProcessor(32)
	entry := Entry{Fields: map[string]any{
		"response_body": map[string]any{"items": []any{strings.Repeat("x", 128)}},
	}}

	got := p.Process(entry)

	responseBody, ok := got.Fields["response_body"].(string)
	require.True(t, ok)
	assert.Len(t, strings.TrimSuffix(responseBody, truncatedSuffix), 32)
	assert.Contains(t, responseBody, "truncated")
}

func TestPipelineTruncatesJSONBodyBeforeRemoveEscape(t *testing.T) {
	buf := &bytes.Buffer{}
	s, err := NewSink(NopWriteCloser{Writer: buf}, "trace", SinkConfig{})
	require.NoError(t, err)
	pipeline := NewPipeline(s,
		NewTruncateBodyProcessor(64),
		NewRemoveEscapeProcessor(),
	)

	largeJSON := `{"items":["` + strings.Repeat("a", 256) + `"]}`
	err = pipeline.Process(Entry{
		Timestamp: time.Unix(0, 0),
		Severity:  Info,
		Level:     "DUMP",
		Fields: map[string]any{
			"response_body": largeJSON,
			"message":       strings.Repeat("c", 256),
		},
	})
	require.NoError(t, err)
	require.NoError(t, s.Close(context.Background()))

	var got map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got))
	responseBody, ok := got["response_body"].(string)
	require.True(t, ok)
	assert.Len(t, strings.TrimSuffix(responseBody, truncatedSuffix), 64)
	assert.Equal(t, strings.Repeat("c", 256), got["message"])
}

func TestPipelineTruncatesBeforeMask(t *testing.T) {
	buf := &bytes.Buffer{}
	s, err := NewSink(NopWriteCloser{Writer: buf}, "trace", SinkConfig{})
	require.NoError(t, err)
	pipeline := NewPipeline(s,
		NewTruncateBodyProcessor(64),
		NewMaskProcessor([]string{"cardNo"}),
		NewRemoveEscapeProcessor(),
	)

	largeJSON := `{"cardNo":"4111111111111111","items":["` + strings.Repeat("a", 256) + `"]}`
	err = pipeline.Process(Entry{
		Timestamp: time.Unix(0, 0),
		Severity:  Info,
		Level:     "DUMP",
		Fields: map[string]any{
			"response_body": largeJSON,
		},
	})
	require.NoError(t, err)
	require.NoError(t, s.Close(context.Background()))

	var got map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got))
	assert.Contains(t, got["response_body"].(string), `"cardNo":"****`)
}

func mustJSONQuote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return string(b)
}
