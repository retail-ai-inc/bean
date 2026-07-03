package http

type LoggingOptions struct {
	DumpBody           bool
	BodyLimit          int64
	LogType            string
	AllowedReqHeaders  []string
	AllowedRespHeaders []string
}
