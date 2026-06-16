package log

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
)

type Processor interface {
	Process(entry Entry) Entry
}

type MaskProcessor struct {
	fields map[string]struct{}
}

func NewMaskProcessor(fields []string) *MaskProcessor {
	fm := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		fm[f] = struct{}{}
	}

	return &MaskProcessor{fields: fm}
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
		if looksLikeJSON(v) {
			var decoded interface{}
			if err := json.Unmarshal([]byte(v), &decoded); err != nil {
				return v
			}
			masked := p.maskValue(decoded)
			b, err := json.Marshal(masked)
			if err != nil {
				return v
			}
			return string(b)
		}
		if looksLikeXML(v) {
			if out, ok := p.maskXMLBytes([]byte(v)); ok {
				return string(out)
			}
		}
		return v

	case json.RawMessage:
		b, ok := p.maskJSONBytes([]byte(v))
		if !ok {
			return v
		}
		return json.RawMessage(b)

	case []byte:
		b, ok := p.maskJSONBytes(v)
		if !ok {
			return string(v)
		}
		return json.RawMessage(b)

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

// looksLikeXML reports whether the first non-whitespace byte is '<', which
// covers XML declarations, SOAP envelopes and plain XML documents.
func looksLikeXML(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case '<':
			return true
		default:
			return false
		}
	}
	return false
}

func (p *MaskProcessor) maskXMLBytes(in []byte) ([]byte, bool) {
	dec := xml.NewDecoder(bytes.NewReader(in))
	dec.Strict = false

	var spans [][2]int
	depth := 0
	maskDepth := 0 // 0 = not masking; >0 = inside a masked subtree rooted at this depth

	for {
		off0 := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false
		}
		off1 := dec.InputOffset()

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if maskDepth == 0 {
				if _, ok := p.fields[t.Name.Local]; ok {
					maskDepth = depth
				}
			}
		case xml.EndElement:
			if maskDepth > 0 && depth == maskDepth {
				maskDepth = 0
			}
			depth--
		case xml.CharData:
			if maskDepth > 0 && len(bytes.TrimSpace([]byte(t))) > 0 {
				spans = append(spans, [2]int{int(off0), int(off1)})
			}
		}
	}

	if len(spans) == 0 {
		return in, true
	}

	var out bytes.Buffer
	prev := 0
	for _, s := range spans {
		if s[0] < prev || s[1] > len(in) {
			continue
		}
		out.Write(in[prev:s[0]])
		out.WriteString("****")
		prev = s[1]
	}
	out.Write(in[prev:])

	return out.Bytes(), true
}
