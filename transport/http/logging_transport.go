package http

import (
	"bytes"
	"github.com/labstack/echo/v4"
	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/retail-ai-inc/bean/v2/logging"
	"io"
	"net/http"
	"time"
)

type LoggingTransport struct {
	Base   http.RoundTripper
	Logger *logging.Logger
	Opt    LoggingOptions
}

func NewLoggingTransport(
	base http.RoundTripper,
	logger *logging.Logger,
	opt LoggingOptions,
) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	if opt.MaxBodySize == 0 {
		opt.MaxBodySize = 64 * 1024
	}

	return &LoggingTransport{
		Base:   base,
		Logger: logger,
		Opt:    opt,
	}
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fields := map[string]any{
		"method": req.Method,
		"url":    req.URL.String(),
	}

	if t.Opt.DumpBody && req != nil && req.Body != nil {
		reqBody, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		fields["request_body"] = string(reqBody)
	}

	reqHeader := make(map[string]any)
	if requestID, ok := bctx.GetRequestID(req.Context()); ok {
		reqHeader[echo.HeaderXRequestID] = requestID
	}
	for _, h := range t.Opt.AllowedHeaders {
		if v := req.Header.Get(h); v != "" {
			reqHeader[h] = v
		}
	}
	if len(reqHeader) > 0 {
		fields["request_header"] = reqHeader
	}

	start := time.Now()

	resp, err := t.Base.RoundTrip(req)

	fields["latency_ms"] = time.Since(start).Milliseconds()

	if resp != nil {
		fields["status"] = resp.StatusCode
	}

	if t.Opt.DumpBody && resp != nil && resp.Body != nil {
		limited := io.LimitReader(resp.Body, t.Opt.MaxBodySize)
		buf := &bytes.Buffer{}
		respBody, _ := io.ReadAll(io.TeeReader(limited, buf))
		resp.Body = io.NopCloser(io.MultiReader(buf, resp.Body))

		fields["response_body"] = string(respBody)
	}

	if err != nil {
		fields["error"] = err.Error()
		t.Logger.Error(req.Context(), "outbound_http", fields)
		return resp, err
	}

	t.Logger.Info(req.Context(), "outbound_http", fields)

	return resp, nil
}
