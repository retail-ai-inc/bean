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
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/color"
	"github.com/valyala/fasttemplate"
)

type (
	// LoggerConfig defines the config for Logger middleware.
	LoggerConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// Optional. Default value DefaultLoggerConfig.AccessLogFormat.
		AccessLogFormat string `yaml:"format"`

		// Optional. Default value DefaultLoggerConfig.BodyDumpFormat.
		BodyDumpFormat string `yaml:"format"`

		// Optional. Default value DefaultLoggerConfig.CustomTimeFormat.
		CustomTimeFormat string `yaml:"custom_time_format"`

		// Output is a writer where logs in JSON format are written.
		// Optional. Default value os.Stdout.
		Output io.Writer

		// BodyDump is an option to control the log also print the request and response body.
		// Optional. Default value false.
		BodyDump bool

		// MaskedParameters is a slice of parameters for which the user wants to mask the value in logs.
		// Optional. Default value [].
		MaskedParameters []string

		// RequestHeader is a slice of HTTP request header parameters which user wants to log.
		RequestHeader []string

		accessLogTemplate *fasttemplate.Template
		bodyDumpTemplate  *fasttemplate.Template
		colorer           *color.Color
		pool              *sync.Pool
	}

	bodyDumpResponseWriter struct {
		io.Writer
		http.ResponseWriter
		Status int
	}
)

var (
	accessLogFormat = `{"time":"${time_rfc3339_nano}","level":"ACCESS","id":"${id}","remote_ip":"${remote_ip}",` +
		`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
		`"X-Forwarded-For":"${header:X-Forwarded-For}","bytes_in":${bytes_in},"request_header":${req_header}}` + "\n"

	bodyDumpFormat = `{"time":"${time_rfc3339_nano}","level":"DUMP","id":"${id}","uri":"${uri}","status":${status},` +
		`"error":"${error}","latency":${latency},"latency_human":"${latency_human}",` +
		`"bytes_in":${bytes_in},"request_body":${request_body},` +
		`"bytes_out":${bytes_out},"response_body":${response_body},"request_header":${req_header}}` + "\n"

	// DefaultLoggerConfig is the default Logger middleware config.
	DefaultLoggerConfig = LoggerConfig{
		Skipper:          middleware.DefaultSkipper,
		AccessLogFormat:  accessLogFormat,
		BodyDumpFormat:   bodyDumpFormat,
		CustomTimeFormat: "2006-01-02 15:04:05.00000",
		colorer:          color.New(),
	}
)

// AccessLoggerWithConfig returns a Logger middleware with config.
func AccessLoggerWithConfig(config LoggerConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultLoggerConfig.Skipper
	}
	if config.AccessLogFormat == "" {
		config.AccessLogFormat = DefaultLoggerConfig.AccessLogFormat
	}
	if config.BodyDumpFormat == "" {
		config.BodyDumpFormat = DefaultLoggerConfig.BodyDumpFormat
	}
	if config.Output == nil {
		config.Output = DefaultLoggerConfig.Output
	}

	config.accessLogTemplate = fasttemplate.New(config.AccessLogFormat, "${", "}")
	config.bodyDumpTemplate = fasttemplate.New(config.BodyDumpFormat, "${", "}")
	config.colorer = color.New()
	config.colorer.SetOutput(config.Output)
	config.pool = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			// Skip the logging if skipper is configured via `skipEndpoints` parameter in env.json.
			if config.Skipper(c) {
				return next(c)
			}

			// Logging into the access log before processing the request.
			if err = config.logAccess(c); err != nil {
				return
			}

			// Skip the body dumper log if `bodyDump == false` means when the body dumper is off.
			if !config.BodyDump {
				return next(c)
			}

			// IMPORTANT: Get a copy of the request body for body dumper.
			reqBody := []byte{}
			if c.Request().Body != nil { // Read
				reqBody, _ = io.ReadAll(c.Request().Body)
			}
			c.Request().Body = io.NopCloser(bytes.NewBuffer(reqBody)) // Reset

			// IMPORTANT: Create a multiWriter writes to both response
			// and the local body dumper buffer. (`resBody` variable below)
			resBody := new(bytes.Buffer)
			mw := io.MultiWriter(c.Response().Writer, resBody)
			writer := &bodyDumpResponseWriter{Writer: mw, ResponseWriter: c.Response().Writer}
			c.Response().Writer = writer

			// Process the request and dump the body with extra information.
			req := c.Request()
			res := c.Response()
			start := time.Now()
			if err = next(c); err != nil {
				c.Error(err)
			}
			stop := time.Now()
			buf := config.pool.Get().(*bytes.Buffer)
			buf.Reset()
			defer config.pool.Put(buf)

			if _, err = config.bodyDumpTemplate.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
				switch tag {
				case "time_unix":
					return buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
				case "time_unix_nano":
					return buf.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
				case "time_rfc3339":
					return buf.WriteString(time.Now().Format(time.RFC3339))
				case "time_rfc3339_nano":
					return buf.WriteString(time.Now().Format(time.RFC3339Nano))
				case "time_custom":
					return buf.WriteString(time.Now().Format(config.CustomTimeFormat))
				case "id":
					id := req.Header.Get(echo.HeaderXRequestID)
					if id == "" {
						id = res.Header().Get(echo.HeaderXRequestID)
					}
					return buf.WriteString(id)
				case "remote_ip":
					return buf.WriteString(c.RealIP())
				case "host":
					return buf.WriteString(req.Host)
				case "uri":
					return buf.WriteString(req.RequestURI)
				case "method":
					return buf.WriteString(req.Method)
				case "path":
					p := req.URL.Path
					if p == "" {
						p = "/"
					}
					return buf.WriteString(p)
				case "protocol":
					return buf.WriteString(req.Proto)
				case "referer":
					return buf.WriteString(req.Referer())
				case "user_agent":
					return buf.WriteString(req.UserAgent())
				case "status":
					n := writer.Status
					s := config.colorer.Green(n)
					switch {
					case n >= 500:
						s = config.colorer.Red(n)
					case n >= 400:
						s = config.colorer.Yellow(n)
					case n >= 300:
						s = config.colorer.Cyan(n)
					}
					return buf.WriteString(s)
				case "error":
					if err != nil {
						// Error may contain invalid JSON e.g. `"`
						b, _ := json.Marshal(err.Error())
						b = b[1 : len(b)-1]
						return buf.Write(b)
					}
				case "latency":
					l := stop.Sub(start)
					return buf.WriteString(strconv.FormatInt(int64(l), 10))
				case "latency_human":
					return buf.WriteString(stop.Sub(start).String())
				case "bytes_in":
					cl := req.Header.Get(echo.HeaderContentLength)
					if cl == "" {
						cl = "0"
					}
					return buf.WriteString(cl)
				case "bytes_out":
					return buf.WriteString(strconv.FormatInt(res.Size, 10))
				case "request_body":
					if len(reqBody) > 0 {
						reqBody, err = maskSensitiveInfo(reqBody, config.MaskedParameters)
						if err == nil {
							return buf.Write(reqBody)
						}
					}
					return buf.WriteString(`null`)
				case "response_body":
					res_body := strings.TrimSuffix(resBody.String(), "\n")
					if res_body != "" {
						return buf.WriteString(res_body)
					}
					return buf.WriteString(`null`)
				case "req_header":
					if len(config.RequestHeader) > 0 {
						reqHeader := make(map[string]interface{})
						for _, param := range config.RequestHeader {
							v := c.Request().Header.Get(param)
							if v != "" {
								reqHeader[param] = v
							}
						}
						reqHeaderByte, err := json.Marshal(reqHeader)
						if err == nil {
							return buf.Write(reqHeaderByte)
						}
					}
					return buf.WriteString(`null`)
				default:
					switch {
					case strings.HasPrefix(tag, "header:"):
						return buf.Write([]byte(c.Request().Header.Get(tag[7:])))
					case strings.HasPrefix(tag, "query:"):
						return buf.Write([]byte(c.QueryParam(tag[6:])))
					case strings.HasPrefix(tag, "form:"):
						return buf.Write([]byte(c.FormValue(tag[5:])))
					case strings.HasPrefix(tag, "cookie:"):
						cookie, err := c.Cookie(tag[7:])
						if err == nil {
							return buf.Write([]byte(cookie.Value))
						}
					}
				}
				return 0, nil
			}); err != nil {
				return
			}

			if config.Output == nil {
				_, err = c.Logger().Output().Write(buf.Bytes())
				return
			}
			_, err = config.Output.Write(buf.Bytes())
			return

		}
	}
}

func (config LoggerConfig) logAccess(c echo.Context) (err error) {
	req := c.Request()
	buf := config.pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer config.pool.Put(buf)
	if _, err = config.accessLogTemplate.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
		switch tag {
		case "time_unix":
			return buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
		case "time_unix_nano":
			return buf.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
		case "time_rfc3339":
			return buf.WriteString(time.Now().Format(time.RFC3339))
		case "time_rfc3339_nano":
			return buf.WriteString(time.Now().Format(time.RFC3339Nano))
		case "time_custom":
			return buf.WriteString(time.Now().Format(config.CustomTimeFormat))
		case "id":
			id := req.Header.Get(echo.HeaderXRequestID)
			return buf.WriteString(id)
		case "remote_ip":
			return buf.WriteString(c.RealIP())
		case "host":
			return buf.WriteString(req.Host)
		case "uri":
			return buf.WriteString(req.RequestURI)
		case "method":
			return buf.WriteString(req.Method)
		case "path":
			p := req.URL.Path
			if p == "" {
				p = "/"
			}
			return buf.WriteString(p)
		case "protocol":
			return buf.WriteString(req.Proto)
		case "referer":
			return buf.WriteString(req.Referer())
		case "user_agent":
			return buf.WriteString(req.UserAgent())
		case "bytes_in":
			cl := req.Header.Get(echo.HeaderContentLength)
			if cl == "" {
				cl = "0"
			}
			return buf.WriteString(cl)
		case "req_header":
			if len(config.RequestHeader) > 0 {
				reqHeader := make(map[string]interface{})
				for _, param := range config.RequestHeader {
					v := c.Request().Header.Get(param)
					if v != "" {
						reqHeader[param] = v
					}
				}
				reqHeaderByte, err := json.Marshal(reqHeader)
				if err == nil {
					return buf.Write(reqHeaderByte)
				}
			}
			return buf.WriteString(`null`)
		default:
			switch {
			case strings.HasPrefix(tag, "header:"):
				return buf.Write([]byte(c.Request().Header.Get(tag[7:])))
			case strings.HasPrefix(tag, "query:"):
				return buf.Write([]byte(c.QueryParam(tag[6:])))
			case strings.HasPrefix(tag, "form:"):
				return buf.Write([]byte(c.FormValue(tag[5:])))
			case strings.HasPrefix(tag, "cookie:"):
				cookie, err := c.Cookie(tag[7:])
				if err == nil {
					return buf.Write([]byte(cookie.Value))
				}
			}
		}
		return 0, nil
	}); err != nil {
		return
	}

	if config.Output == nil {
		_, err = c.Logger().Output().Write(buf.Bytes())
		return
	}
	_, err = config.Output.Write(buf.Bytes())
	return
}

func (w *bodyDumpResponseWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyDumpResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *bodyDumpResponseWriter) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *bodyDumpResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

func maskSensitiveInfo(reqBody []byte, maskedParams []string) ([]byte, error) {
	if len(maskedParams) == 0 {
		return reqBody, nil
	}

	var unmarshaledRequest = make(map[string]interface{})
	err := json.Unmarshal(reqBody, &unmarshaledRequest)
	if err != nil {
		return reqBody, err
	}

	for _, maskedParam := range maskedParams {
		if _, ok := unmarshaledRequest[maskedParam]; ok {
			unmarshaledRequest[maskedParam] = "****"
		}
	}
	maskedRequestBody, _ := json.Marshal(unmarshaledRequest)

	return maskedRequestBody, nil
}
