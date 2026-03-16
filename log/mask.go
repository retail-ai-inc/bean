package log

import (
	"encoding/json"
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
	if entry.Fields == nil {
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

	case json.RawMessage, []byte:
		var decoded interface{}
		if err := json.Unmarshal(v.([]byte), &decoded); err != nil {
			return string(v.([]byte))
		}
		masked := p.maskValue(decoded)
		b, err := json.Marshal(masked)
		if err != nil {
			return string(v.([]byte))
		}
		return json.RawMessage(b)

	default:
		return v
	}
}
