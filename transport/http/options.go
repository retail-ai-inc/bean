package http

type LoggingOptions struct {
	DumpBody       bool
	MaxBodySize    int64
	AllowedHeaders []string
}
