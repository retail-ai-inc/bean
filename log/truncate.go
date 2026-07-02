package log

import (
	"encoding/json"
	"unicode/utf8"
)

const (
	// DefaultMaxSizeBytes limits one encoded structured log entry to 8KB.
	DefaultMaxSizeBytes = 8 * 1024
	truncatedSuffix     = "...(truncated)"
)

type truncateCandidate struct {
	value string
	set   func(string)
}

func encodedPayloadSize(payload map[string]any) int {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	return len(b) + 1 // json.Encoder appends a newline.
}

func truncatePayloadToSize(payload map[string]any, maxBytes int) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSizeBytes
	}

	for range 128 {
		currentSize := encodedPayloadSize(payload)
		if currentSize == 0 || currentSize <= maxBytes {
			return
		}

		candidate, ok := largestTruncateCandidate(payload)
		if !ok {
			return
		}

		overBy := currentSize - maxBytes
		targetBytes := len(candidate.value) - overBy
		if targetBytes >= len(candidate.value) {
			targetBytes = len(candidate.value) / 2
		}
		if targetBytes < len(truncatedSuffix) {
			targetBytes = len(truncatedSuffix)
		}

		next := truncateStringBytes(candidate.value, targetBytes)
		if next == candidate.value {
			next = truncatedSuffix
		}
		candidate.set(next)
	}
}

func largestTruncateCandidate(v any) (truncateCandidate, bool) {
	var best truncateCandidate
	var found bool
	collectTruncateCandidates(v, func(c truncateCandidate) {
		if !found || len(c.value) > len(best.value) {
			best = c
			found = true
		}
	})
	return best, found
}

func collectTruncateCandidates(v any, add func(truncateCandidate)) {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			key := k
			collectTruncateCandidatesWithSetter(vv, func(next any) { x[key] = next }, add)
		}
	case []any:
		for i, vv := range x {
			idx := i
			collectTruncateCandidatesWithSetter(vv, func(next any) { x[idx] = next }, add)
		}
	}
}

func collectTruncateCandidatesWithSetter(v any, setAny func(any), add func(truncateCandidate)) {
	switch x := v.(type) {
	case string:
		if x != truncatedSuffix {
			add(truncateCandidate{value: x, set: func(next string) { setAny(next) }})
		}
	case []byte:
		s := string(x)
		if s != truncatedSuffix {
			add(truncateCandidate{value: s, set: func(next string) { setAny(next) }})
		}
	case json.RawMessage:
		s := string(x)
		if s != truncatedSuffix {
			add(truncateCandidate{value: s, set: func(next string) { setAny(next) }})
		}
	case map[string]any:
		collectTruncateCandidates(x, add)
	case []any:
		collectTruncateCandidates(x, add)
	}
}

func truncateStringBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSizeBytes
	}
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes <= len(truncatedSuffix) {
		return trimStringToBytes(truncatedSuffix, maxBytes)
	}

	prefixLimit := maxBytes - len(truncatedSuffix)
	return trimStringToBytes(s, prefixLimit) + truncatedSuffix
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
