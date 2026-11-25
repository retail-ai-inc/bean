// Package json exposes the JSON functions used by bean.
//
// Deprecated: use encoding/json. This package used github.com/json-iterator/go, which is no longer maintained; it
// now forwards to encoding/json (backed by the faster json v2 implementation since Go 1.27).
package json

import "encoding/json"

var (
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
