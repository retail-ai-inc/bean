package log

import (
	"encoding/json"
	"unicode/utf8"
)

const (
	// DefaultMaxSizeBytes limits request_body and response_body fields to 8KB each.
	DefaultMaxSizeBytes = 8 * 1024
	truncatedSuffix     = "..."
)

var bodyLogFields = [...]string{"request_body", "response_body"}

type TruncateBodyProcessor struct {
	maxBytes int
}

func NewTruncateBodyProcessor(maxBytes int) *TruncateBodyProcessor {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSizeBytes
	}
	return &TruncateBodyProcessor{maxBytes: maxBytes}
}

func (p *TruncateBodyProcessor) Process(entry Entry) Entry {
	if entry.Fields == nil {
		return entry
	}
	truncateBodyFields(entry.Fields, p.maxBytes)
	return entry
}

func truncateBodyFields(payload map[string]any, maxBytes int) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSizeBytes
	}

	for _, field := range bodyLogFields {
		value, ok := payload[field]
		if !ok {
			continue
		}
		payload[field] = truncateBodyValue(value, maxBytes)
	}
}

func truncateBodyValue(value any, maxBytes int) any {
	switch v := value.(type) {
	case string:
		return truncateStringBytes(v, maxBytes)
	case []byte:
		if len(v) <= maxBytes {
			return v
		}
		return truncateStringBytes(string(v), maxBytes)
	case json.RawMessage:
		if len(v) <= maxBytes {
			return v
		}
		// Return a string instead of RawMessage so truncation cannot produce invalid JSON.
		return truncateStringBytes(string(v), maxBytes)
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return v
		}
		return truncateStringBytes(string(b), maxBytes)
	default:
		return v
	}
}

func truncateStringBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSizeBytes
	}
	if len(s) <= maxBytes {
		return s
	}
	return trimStringToBytes(s, maxBytes) + truncatedSuffix
}

func trimStringToBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	end := 0
	for end < len(s) {
		r, size := utf8.DecodeRuneInString(s[end:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		if end+size > maxBytes {
			break
		}
		end += size
	}

	return s[:end]
}
