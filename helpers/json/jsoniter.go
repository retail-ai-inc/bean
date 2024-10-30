package json

import jsoniter "github.com/json-iterator/go"

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
	// Marshal is exported by bean/json package.
	Marshal = json.Marshal
	// Unmarshal is exported by bean/json package.
	Unmarshal = json.Unmarshal
	// MarshalIndent is exported by bean/json package.
	MarshalIndent = json.MarshalIndent
	// NewDecoder is exported by bean/json package.
	NewDecoder = json.NewDecoder
	// NewEncoder is exported by bean/json package.
	NewEncoder = json.NewEncoder
)
