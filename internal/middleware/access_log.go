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
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/retail-ai-inc/bean/v2/log"
)

type LoggerConfig struct {
	Skipper        middleware.Skipper
	BodyDump       bool
	RequestHeader  []string
	ResponseHeader []string
	Logger         log.BeanLogger
}

type bodyDumpResponseWriter struct {
	io.Writer
	http.ResponseWriter
	Status int
}

var DefaultLoggerConfig = LoggerConfig{
	Skipper: middleware.DefaultSkipper,
}

func AccessLoggerWithConfig(config LoggerConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultLoggerConfig.Skipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
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
			var reqBody []byte
			if config.BodyDump && req.Body != nil {
				reqBody, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			var resBody *bytes.Buffer
			var writer *bodyDumpResponseWriter

			if config.BodyDump {
				resBody = new(bytes.Buffer)
				mw := io.MultiWriter(res.Writer, resBody)
				writer = &bodyDumpResponseWriter{
					Writer:         mw,
					ResponseWriter: res.Writer,
				}
				res.Writer = writer
			}

			// ---- Execute handler ----
			if err = next(c); err != nil {
				c.Error(err)
			}

			stop := time.Now()

			// ---- Body Dump Log (after) ----
			if config.BodyDump {
				logBodyDump(config, c, reqBody, resBody, writer, start, stop, err)
			}

			return
		}
	}
}

func logAccess(config LoggerConfig, c echo.Context) {
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
	c echo.Context,
	reqBody []byte,
	resBody *bytes.Buffer,
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
		"bytes_out":     res.Size,
	}

	if handlerErr != nil {
		fields["error"] = handlerErr.Error()
	}

	// ---- structured request body ----
	if len(reqBody) > 0 {
		fields["request_body"] = reqBody
	}

	// ---- structured response body ----
	if resBody != nil && resBody.Len() > 0 {
		fields["response_body"] = resBody.Bytes()
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

func (w *bodyDumpResponseWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyDumpResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
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
