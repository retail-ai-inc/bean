package http

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/config"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/retail-ai-inc/bean/v2/internal/redact"
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

	if opt.BodyLimit == 0 {
		opt.BodyLimit = 64 * 1024
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

	reqHeader := redact.Headers(req.Header, t.opt.AllowedReqHeaders)
	if requestID, ok := bctx.GetRequestID(req.Context()); ok {
		fields["id"] = requestID
		// A non-empty X-Request-ID header on the request itself takes precedence.
		if _, exists := reqHeader[echo.HeaderXRequestID]; !exists {
			reqHeader[echo.HeaderXRequestID] = requestID
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
		respHeader := redact.Headers(resp.Header, t.opt.AllowedRespHeaders)

		if len(respHeader) > 0 {
			fields["response_header"] = respHeader
		}
	}

	if t.opt.DumpBody && resp != nil && resp.Body != nil {
		limited := io.LimitReader(resp.Body, t.opt.BodyLimit)
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
