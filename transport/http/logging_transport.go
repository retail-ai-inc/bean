package http

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	blog "github.com/retail-ai-inc/bean/v2/log"
)

type LoggingTransport struct {
	base   http.RoundTripper
	logger blog.BeanLogger
	opt    LoggingOptions
}

func NewLoggingTransport(
	base http.RoundTripper,
	logger blog.BeanLogger,
	opt LoggingOptions,
) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	if opt.MaxBodySize == 0 {
		opt.MaxBodySize = 64 * 1024
	}

	return &LoggingTransport{
		base:   base,
		logger: logger,
		opt:    opt,
	}
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fields := map[string]any{
		"method": req.Method,
		"url":    req.URL.String(),
	}

	if t.opt.DumpBody && req != nil && req.Body != nil {
		reqBody, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		fields["request_body"] = reqBody
	}

	if t.opt.LogType != "" {
		fields["type"] = t.opt.LogType
	}

	reqHeader := make(map[string]any)
	if requestID, ok := bctx.GetRequestID(req.Context()); ok {
		reqHeader[echo.HeaderXRequestID] = requestID
	}
	for _, h := range t.opt.AllowedHeaders {
		if v := req.Header.Get(h); v != "" {
			reqHeader[h] = v
		}
	}
	if len(reqHeader) > 0 {
		fields["request_header"] = reqHeader
	}

	start := time.Now()

	resp, err := t.base.RoundTrip(req)

	fields["latency_ms"] = time.Since(start).Milliseconds()

	if resp != nil {
		fields["status"] = resp.StatusCode
	}

	if t.opt.DumpBody && resp != nil && resp.Body != nil {
		limited := io.LimitReader(resp.Body, t.opt.MaxBodySize)
		buf := &bytes.Buffer{}
		respBody, _ := io.ReadAll(io.TeeReader(limited, buf))
		resp.Body = io.NopCloser(io.MultiReader(buf, resp.Body))

		fields["response_body"] = respBody
	}

	if err != nil {
		fields["error"] = err.Error()
		t.logger.TraceError(req.Context(), "outbound_http", fields)
		return resp, err
	}

	t.logger.TraceInfo(req.Context(), "outbound_http", fields)

	return resp, nil
}
