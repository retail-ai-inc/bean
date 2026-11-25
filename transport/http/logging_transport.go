package http

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/retail-ai-inc/bean/v3/config"
	bctx "github.com/retail-ai-inc/bean/v3/context"
	blog "github.com/retail-ai-inc/bean/v3/log"
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

	// Default to the access log headers of the bean config, if it is loaded.
	if config.Bean != nil {
		if len(opt.AllowedReqHeaders) == 0 {
			opt.AllowedReqHeaders = config.Bean.AccessLog.ReqHeaderParam
		}
		if len(opt.AllowedRespHeaders) == 0 {
			opt.AllowedRespHeaders = config.Bean.AccessLog.ResHeaderParam
		}
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

	if t.opt.DumpBody && req.Body != nil && req.Body != http.NoBody {
		reqBody, newReq, err := t.dumpRequestBody(req)
		if err != nil {
			return nil, err
		}
		req = newReq
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

	// A 101 response body is the upgraded connection (an io.ReadWriteCloser): it must not be wrapped.
	if t.opt.DumpBody && resp != nil && resp.Body != nil && resp.StatusCode != http.StatusSwitchingProtocols {
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, t.opt.BodyLimit))
		// The caller reads the logged prefix and then the rest of the body; closing it closes the original body
		// (otherwise the connection is never released).
		rest := io.Reader(resp.Body)
		if readErr != nil {
			rest = errReader{readErr}
		}
		resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(respBody), rest), Closer: resp.Body}

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

// dumpRequestBody returns the first `BodyLimit` bytes of the request body, and the request to send.
//
// With `GetBody` (set by http.NewRequest for in-memory bodies) the prefix is read from a copy and the request is
// unchanged. Otherwise the prefix is read from the body, and a copy of the request is returned whose body replays
// the prefix then the rest of the original body (a RoundTripper must not modify the caller's request).
func (t *LoggingTransport) dumpRequestBody(req *http.Request) ([]byte, *http.Request, error) {
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err == nil {
			prefix, readErr := io.ReadAll(io.LimitReader(body, t.opt.BodyLimit))
			_ = body.Close()
			if readErr == nil {
				return prefix, req, nil
			}
		}
	}

	prefix, err := io.ReadAll(io.LimitReader(req.Body, t.opt.BodyLimit))
	if err != nil {
		// Do not send a truncated body. RoundTrip must close the body, even on error.
		_ = req.Body.Close()
		return nil, nil, err
	}

	newReq := req.Clone(req.Context())
	newReq.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(prefix), req.Body), Closer: req.Body}
	return prefix, newReq, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

// errReader returns err on every read.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
