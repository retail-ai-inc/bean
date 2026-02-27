package http

import (
	"bytes"
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
	start := time.Now()

	var reqBody []byte
	if t.Opt.DumpBody && req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	}

	resp, err := t.Base.RoundTrip(req)
	latency := time.Since(start)

	fields := map[string]any{
		"http": map[string]any{
			"method":     req.Method,
			"url":        req.URL.String(),
			"latency_ms": latency.Milliseconds(),
		},
	}

	if len(t.Opt.AllowedHeaders) > 0 {
		reqHeader := make(map[string]any)
		for _, h := range t.Opt.AllowedHeaders {
			if v := req.Header.Get(h); v != "" {
				reqHeader[h] = v
			}
		}
		fields["http"].(map[string]any)["request_header"] = reqHeader
	}

	if err != nil {
		fields["error"] = err.Error()
		t.Logger.Error(req.Context(), "outbound_http", fields)
		return resp, err
	}

	fields["http"].(map[string]any)["status"] = resp.StatusCode

	if t.Opt.DumpBody && resp != nil && resp.Body != nil {
		limited := io.LimitReader(resp.Body, t.Opt.MaxBodySize)
		buf := &bytes.Buffer{}
		respBody, _ := io.ReadAll(io.TeeReader(limited, buf))
		resp.Body = io.NopCloser(io.MultiReader(buf, resp.Body))

		fields["request_body"] = string(reqBody)
		fields["response_body"] = string(respBody)
	}

	t.Logger.Info(req.Context(), "outbound_http", fields)

	return resp, nil
}
