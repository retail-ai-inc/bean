package http

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/config"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	blog "github.com/retail-ai-inc/bean/v2/log"
)

type LoggingTransport struct {
	base   http.RoundTripper
	logger blog.AccessLogger
	opt    LoggingOptions
}

func NewLoggingTransport(
	base http.RoundTripper,
	logger blog.AccessLogger,
	opt LoggingOptions,
) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	if opt.MaxBodySize == 0 {
		opt.MaxBodySize = 64 * 1024
	}

	if len(opt.AllowedReqHeaders) == 0 {
		opt.AllowedReqHeaders = config.Bean.AccessLog.ReqHeaderParam
	}

	if len(opt.AllowedRespHeaders) == 0 {
		opt.AllowedRespHeaders = config.Bean.AccessLog.ResHeaderParam
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
		fields["request_body"] = string(reqBody)
	}

	if t.opt.LogType != "" {
		fields["type"] = t.opt.LogType
	}

	reqHeader := make(map[string]any)
	if requestID, ok := bctx.GetRequestID(req.Context()); ok {
		fields["id"] = requestID
		reqHeader[echo.HeaderXRequestID] = requestID
	}
	for _, h := range t.opt.AllowedReqHeaders {
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
		respHeader := make(map[string]any)
		for _, h := range t.opt.AllowedRespHeaders {
			if v := resp.Header.Get(h); v != "" {
				respHeader[h] = v
			}
		}

		if len(respHeader) > 0 {
			fields["response_header"] = respHeader
		}
	}

	if t.opt.DumpBody && resp != nil && resp.Body != nil {
		limited := io.LimitReader(resp.Body, t.opt.MaxBodySize)
		buf := &bytes.Buffer{}
		respBody, _ := io.ReadAll(io.TeeReader(limited, buf))
		resp.Body = io.NopCloser(io.MultiReader(buf, resp.Body))

		fields["response_body"] = string(respBody)
	}

	if err != nil {
		fields["error"] = err.Error()
		t.logger.TraceError(req.Context(), "OUTBOUND_API", fields)
		return resp, err
	}

	t.logger.TraceInfo(req.Context(), "OUTBOUND_API", fields)

	return resp, nil
}
