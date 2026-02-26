package processors

import (
	"encoding/json"

	"github.com/retail-ai-inc/bean/v2/logging/types"
)

type RemoveEscapeProcessor struct{}

func NewRemoveEscapeProcessor() *RemoveEscapeProcessor {
	return &RemoveEscapeProcessor{}
}

func (p *RemoveEscapeProcessor) Process(entry types.Entry) types.Entry {
	if entry.Fields == nil {
		return entry
	}

	processed := p.removeEscapeValue(entry.Fields)
	if m, ok := processed.(map[string]interface{}); ok {
		entry.Fields = m
	}

	return entry
}

func (p *RemoveEscapeProcessor) removeEscapeValue(val interface{}) interface{} {
	switch v := val.(type) {
	case string:
		var decoded interface{}
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return p.removeEscapeValue(decoded)
		}
		return v

	case json.RawMessage:
		var decoded interface{}
		if err := json.Unmarshal(v, &decoded); err != nil {
			return v
		}
		processed := p.removeEscapeValue(decoded)
		b, err := json.Marshal(processed)
		if err != nil {
			return v
		}
		return json.RawMessage(b)

	case map[string]interface{}:
		for k, vv := range v {
			v[k] = p.removeEscapeValue(vv)
		}
		return v

	case []interface{}:
		for i, vv := range v {
			v[i] = p.removeEscapeValue(vv)
		}
		return v

	default:
		return v
	}
}
