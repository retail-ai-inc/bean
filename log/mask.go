package log

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// utf8BOM is the UTF-8 byte order mark. We only support UTF-8 payloads; a
// leading BOM is still valid UTF-8 but would defeat the JSON/XML sniffing
// below, so we strip it before detection and masking.
const utf8BOM = "\uFEFF"

type Processor interface {
	Process(entry Entry) Entry
}

type MaskProcessor struct {
	fields map[string]struct{}
	xmlRes []*regexp.Regexp
}

func NewMaskProcessor(fields []string) *MaskProcessor {
	fm := make(map[string]struct{}, len(fields))
	xmlRes := make([]*regexp.Regexp, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		fm[f] = struct{}{}

		quoted := regexp.QuoteMeta(f)
		xmlRes = append(xmlRes, regexp.MustCompile(fmt.Sprintf(`(?s)(<(?:\w+:)?%s\b[^>]*>)(.*?)(</(?:\w+:)?%s>)`, quoted, quoted)))
	}

	return &MaskProcessor{fields: fm, xmlRes: xmlRes}
}

func (p *MaskProcessor) Process(entry Entry) Entry {
	if len(p.fields) == 0 || entry.Fields == nil {
		return entry
	}

	masked := p.maskValue(entry.Fields)
	if m, ok := masked.(map[string]interface{}); ok {
		entry.Fields = m
	}

	return entry
}

func (p *MaskProcessor) maskValue(val interface{}) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		for k, vv := range v {
			if _, ok := p.fields[k]; ok {
				v[k] = "****"
			} else {
				v[k] = p.maskValue(vv)
			}
		}
		return v

	case []interface{}:
		for i, vv := range v {
			v[i] = p.maskValue(vv)
		}
		return v

	case string:
		s := strings.TrimPrefix(v, utf8BOM)
		if !p.containsAnyField(s) {
			return v
		}

		if looksLikeJSON(s) {
			var decoded interface{}
			if err := json.Unmarshal([]byte(s), &decoded); err != nil {
				return v
			}
			masked := p.maskValue(decoded)
			b, err := json.Marshal(masked)
			if err != nil {
				return v
			}
			return string(b)
		}

		if looksLikeXMLPayload(s) {
			if out, masked := p.maskXMLString(s); masked {
				return out
			}
			return "****"
		}
		return v

	case json.RawMessage:
		b, ok := p.maskJSONBytes([]byte(v))
		if !ok {
			return v
		}
		return json.RawMessage(b)

	case []byte:
		s := strings.TrimPrefix(string(v), utf8BOM)
		if !p.containsAnyField(s) {
			return string(v)
		}

		if looksLikeJSON(s) {
			if b, ok := p.maskJSONBytes([]byte(s)); ok {
				return json.RawMessage(b)
			}
			return string(v)
		}

		if looksLikeXMLPayload(s) {
			if out, masked := p.maskXMLString(s); masked {
				return []byte(out)
			}
			// Fail closed for truncated/malformed XML carrying a sensitive field.
			return []byte("****")
		}
		return string(v)

	default:
		return v
	}
}

func (p *MaskProcessor) maskJSONBytes(in []byte) ([]byte, bool) {
	var decoded interface{}
	if err := json.Unmarshal(in, &decoded); err != nil {
		return nil, false
	}
	masked := p.maskValue(decoded)
	out, err := json.Marshal(masked)
	if err != nil {
		return nil, false
	}
	return out, true
}

// looksLikeJSON checks the first byte for JSON structural characters ({, [, ").
func looksLikeJSON(s string) bool {
	if len(s) == 0 {
		return false
	}
	switch s[0] {
	case '{', '[', '"':
		return true
	}
	return false
}

// containsAnyField reports whether any configured mask field name appears as a
// substring of s.
func (p *MaskProcessor) containsAnyField(s string) bool {
	for f := range p.fields {
		if f != "" && strings.Contains(s, f) {
			return true
		}
	}
	return false
}

// looksLikeXMLPayload reports whether s contains XML markup. It intentionally
// tolerates surrounding log noise such as HTTP status lines and headers.
func looksLikeXMLPayload(s string) bool {
	return strings.IndexByte(s, '<') >= 0
}

func (p *MaskProcessor) maskXMLBytes(in []byte) ([]byte, bool) {
	out, masked := p.maskXMLString(string(in))
	if !masked {
		return in, !p.containsAnyField(string(in))
	}
	return []byte(out), true
}

// maskXMLString redacts configured XML/SOAP element values without requiring the
// whole input to be a valid XML document, matching the logger used by emoney.
func (p *MaskProcessor) maskXMLString(s string) (string, bool) {
	if s == "" || !looksLikeXMLPayload(s) || !p.containsAnyField(s) {
		return s, false
	}

	masked := false
	for _, re := range p.xmlRes {
		next := re.ReplaceAllString(s, "${1}****${3}")
		if next != s {
			masked = true
			s = next
		}
	}

	return s, masked
}
