// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package middleware

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/retail-ai-inc/bean/v3/log"
)

type LoggerConfig struct {
	Skipper        middleware.Skipper
	BodyDump       bool
	RequestHeader  []string
	ResponseHeader []string
	Logger         log.AccessLogger
	// BodyLimit is the maximum number of bytes of the request / response body kept in memory for the
	// body dump log. The access log truncates bodies to the same limit, so nothing more is ever logged.
	// <= 0 means `log.DefaultBodyLimit`.
	BodyLimit int
}

// bodyCapture keeps at most `limit` bytes of the data passed through it and counts the total size.
type bodyCapture struct {
	buf   bytes.Buffer
	limit int
	total int64
}

func (b *bodyCapture) capture(p []byte) {
	b.total += int64(len(p))
	if room := b.limit - b.buf.Len(); room > 0 {
		if len(p) > room {
			p = p[:room]
		}
		b.buf.Write(p)
	}
}

// requestBodyRecorder counts the bytes read from the request body and records the first read error
// (e.g. the client aborted the upload).
type requestBodyRecorder struct {
	io.Reader
	read int64
	err  error
}

func (r *requestBodyRecorder) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += int64(n)
	if err != nil && !errors.Is(err, io.EOF) && r.err == nil {
		r.err = err
	}
	return n, err
}

type bodyDumpResponseWriter struct {
	http.ResponseWriter
	body   *bodyCapture
	Status int
}

var DefaultLoggerConfig = LoggerConfig{
	Skipper: middleware.DefaultSkipper,
}

func AccessLoggerWithConfig(config LoggerConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultLoggerConfig.Skipper
	}
	bodyLimit := config.BodyLimit
	if bodyLimit <= 0 {
		bodyLimit = log.DefaultBodyLimit
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			if config.Skipper(c) {
				return next(c)
			}

			if config.Logger == nil {
				return next(c)
			}

			req := c.Request()
			res := c.Response()

			start := time.Now()

			// Access Log (before)
			logAccess(config, c)

			// ---- Body Dump handling ----
			// Only the first `bodyLimit + 1` bytes of the bodies are kept in memory (one extra byte so that the
			// access log can tell the body was truncated); the rest is streamed to / from the handler as is.
			var reqRecorder *requestBodyRecorder
			var reqPrefix []byte
			var writer *bodyDumpResponseWriter

			if config.BodyDump {
				captureLimit := bodyLimit + 1

				if req.Body != nil {
					origBody := req.Body
					reqRecorder = &requestBodyRecorder{Reader: origBody}
					// Read the head of the body now so it is logged even if the handler does not read it.
					reqPrefix, _ = io.ReadAll(io.LimitReader(reqRecorder, int64(captureLimit)))
					req.Body = struct {
						io.Reader
						io.Closer
					}{
						Reader: io.MultiReader(bytes.NewReader(reqPrefix), reqRecorder),
						Closer: origBody,
					}
				}

				writer = &bodyDumpResponseWriter{
					ResponseWriter: res,
					body:           &bodyCapture{limit: captureLimit},
				}
				c.SetResponse(writer)
				defer c.SetResponse(res)
			}

			// ---- Execute handler ----
			if err = next(c); err != nil {
				// Let the global error handler write the error response now so that the
				// response status/body can be logged. (echo v5 removed `c.Error()`.)
				c.Echo().HTTPErrorHandler(c, err)
			}

			stop := time.Now()

			// ---- Body Dump Log (after) ----
			if config.BodyDump {
				logBodyDump(config, c, reqPrefix, reqRecorder, writer, start, stop, err)
			}

			return
		}
	}
}

func logAccess(config LoggerConfig, c *echo.Context) {
	req := c.Request()

	fields := map[string]any{
		"id":         req.Header.Get(echo.HeaderXRequestID),
		"remote_ip":  c.RealIP(),
		"host":       req.Host,
		"method":     req.Method,
		"uri":        req.RequestURI,
		"user_agent": req.UserAgent(),
		"bytes_in":   req.Header.Get(echo.HeaderContentLength),
	}

	if len(config.RequestHeader) > 0 {
		reqHeader := make(map[string]any)
		for _, h := range config.RequestHeader {
			if v := req.Header.Get(h); v != "" {
				reqHeader[h] = v
			}
		}
		fields["request_header"] = reqHeader
	}

	config.Logger.TraceInfo(
		req.Context(),
		"ACCESS",
		fields,
	)
}

func logBodyDump(
	config LoggerConfig,
	c *echo.Context,
	reqBody []byte,
	reqRecorder *requestBodyRecorder,
	writer *bodyDumpResponseWriter,
	start, stop time.Time,
	handlerErr error,
) {
	req := c.Request()
	res := c.Response()

	fields := map[string]any{
		"id":            req.Header.Get(echo.HeaderXRequestID),
		"uri":           req.RequestURI,
		"status":        writer.Status,
		"latency":       int64(stop.Sub(start)),
		"latency_human": stop.Sub(start).String(),
		"bytes_in":      req.Header.Get(echo.HeaderContentLength),
	}

	if r, _ := echo.UnwrapResponse(res); r != nil {
		fields["bytes_out"] = r.Size
	}

	if handlerErr != nil {
		fields["error"] = handlerErr.Error()
	}

	// Keep partial body when the client aborts mid-read (e.g. context canceled).
	if len(reqBody) > 0 {
		fields["request_body"] = string(reqBody)
	}
	if reqRecorder != nil && reqRecorder.err != nil {
		fields["request_body_error"] = reqRecorder.err.Error()
		fields["bytes_in_read"] = reqRecorder.read
	}

	// ---- structured response body ----
	if resBody := &writer.body.buf; resBody.Len() > 0 {
		if isGzipEncoded(res.Header().Get(echo.HeaderContentEncoding)) {
			fields["response_body"] = "[gzip compressed response omitted]"
			fields["response_body_size"] = writer.body.total
			fields["response_content_encoding"] = res.Header().Get(echo.HeaderContentEncoding)
		} else {
			fields["response_body"] = resBody.String()
		}
	}

	// ---- request headers ----
	if len(config.RequestHeader) > 0 {
		reqHeader := make(map[string]any)
		for _, h := range config.RequestHeader {
			if v := req.Header.Get(h); v != "" {
				reqHeader[h] = v
			}
		}
		fields["request_header"] = reqHeader
	}

	// ---- response headers ----
	if len(config.ResponseHeader) > 0 {
		resHeader := make(map[string]any)
		for _, h := range config.ResponseHeader {
			if v := res.Header().Get(h); v != "" {
				resHeader[h] = v
			}
		}
		fields["response_header"] = resHeader
	}

	config.Logger.TraceInfo(
		req.Context(),
		"DUMP",
		fields,
	)
}

func isGzipEncoded(contentEncoding string) bool {
	for _, encoding := range strings.Split(contentEncoding, ",") {
		if strings.EqualFold(strings.TrimSpace(encoding), "gzip") {
			return true
		}
	}
	return false
}

func (w *bodyDumpResponseWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyDumpResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.body.capture(b[:n])
	return n, err
}

// Unwrap returns the original http.ResponseWriter so that `echo.UnwrapResponse` and
// `http.ResponseController` can reach the underlying writer.
func (w *bodyDumpResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *bodyDumpResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *bodyDumpResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}
