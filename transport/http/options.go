package http

type LoggingOptions struct {
	DumpBody           bool
	MaxBodySize        int64
	LogType            string
	AllowedReqHeaders  []string
	AllowedRespHeaders []string
}
