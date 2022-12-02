package json

import jsoniter "github.com/json-iterator/go"

var (
	Json       = jsoniter.ConfigCompatibleWithStandardLibrary
	Marshal    = Json.Marshal
	Unmarshal  = Json.Unmarshal
	NewDecoder = Json.NewDecoder
	NewEncoder = Json.NewEncoder
)
