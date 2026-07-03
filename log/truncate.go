package log

import (
	"encoding/json"
	"unicode/utf8"
)

const (
	// DefaultBodyLimit limits request_body and response_body fields to 8KB each.
	DefaultBodyLimit = 8 * 1024
	truncatedSuffix     = "...(truncated)"
)

var bodyLogFields = [...]string{"request_body", "response_body"}

type TruncateBodyProcessor struct {
	bodyLimit int
}

func NewTruncateBodyProcessor(bodyLimit int) *TruncateBodyProcessor {
	if bodyLimit <= 0 {
		bodyLimit = DefaultBodyLimit
	}
	return &TruncateBodyProcessor{bodyLimit: bodyLimit}
}

func (p *TruncateBodyProcessor) Process(entry Entry) Entry {
	if entry.Fields == nil {
		return entry
	}
	truncateBodyFields(entry.Fields, p.bodyLimit)
	return entry
}

func truncateBodyFields(payload map[string]any, bodyLimit int) {
	if bodyLimit <= 0 {
		bodyLimit = DefaultBodyLimit
	}

	for _, field := range bodyLogFields {
		value, ok := payload[field]
		if !ok {
			continue
		}
		payload[field] = truncateBodyValue(value, bodyLimit)
	}
}

func truncateBodyValue(value any, bodyLimit int) any {
	switch v := value.(type) {
	case string:
		return truncateStringBytes(v, bodyLimit)
	case []byte:
		if len(v) <= bodyLimit {
			return v
		}
		return truncateStringBytes(string(v), bodyLimit)
	case json.RawMessage:
		if len(v) <= bodyLimit {
			return v
		}
		// Return a string instead of RawMessage so truncation cannot produce invalid JSON.
		return truncateStringBytes(string(v), bodyLimit)
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return v
		}
		return truncateStringBytes(string(b), bodyLimit)
	default:
		return v
	}
}

func truncateStringBytes(s string, bodyLimit int) string {
	if bodyLimit <= 0 {
		bodyLimit = DefaultBodyLimit
	}
	if len(s) <= bodyLimit {
		return s
	}
	return trimStringToBytes(s, bodyLimit) + truncatedSuffix
}

func trimStringToBytes(s string, bodyLimit int) string {
	if bodyLimit <= 0 {
		return ""
	}
	if len(s) <= bodyLimit {
		return s
	}

	end := 0
	for end < len(s) {
		r, size := utf8.DecodeRuneInString(s[end:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		if end+size > bodyLimit {
			break
		}
		end += size
	}
	return s[:end]
}
