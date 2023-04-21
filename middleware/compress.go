package middleware

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean"
)

type (
	// GzipConfig defines the config for Gzip middleware.
	GzipConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper Skipper

		// Gzip compression level.
		// Optional. Default value -1.
		Level int `yaml:"level"`
	}

	gzipResponseWriter struct {
		bean.ResponseWriter
		writer    io.Writer
		wroteBody bool
	}
)

const (
	gzipScheme = "gzip"
)

var (
	// DefaultGzipConfig is the default Gzip middleware config.
	DefaultGzipConfig = GzipConfig{
		Skipper: DefaultSkipper,
		Level:   -1,
	}
)

// Gzip returns a middleware which compresses HTTP response using gzip compression
// scheme.
func Gzip() bean.MiddlewareFunc {
	return GzipWithConfig(DefaultGzipConfig)
}

// GzipWithConfig return Gzip middleware with config.
// See: `Gzip()`.
func GzipWithConfig(config GzipConfig) bean.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultGzipConfig.Skipper
	}
	if config.Level == 0 {
		config.Level = DefaultGzipConfig.Level
	}

	pool := gzipCompressPool(config)

	return func(next bean.HandlerFunc) bean.HandlerFunc {
		return func(c bean.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			res := c.Response()
			res.Header().Add(bean.HeaderVary, bean.HeaderAcceptEncoding)
			if strings.Contains(c.Request().Header.Get(bean.HeaderAcceptEncoding), gzipScheme) {
				res.Header().Set(bean.HeaderContentEncoding, gzipScheme) // Issue #806
				i := pool.Get()
				w, ok := i.(*gzip.Writer)
				if !ok {
					return echo.NewHTTPError(http.StatusInternalServerError, i.(error).Error())
				}
				w.Reset(res)
				grw := &gzipResponseWriter{ResponseWriter: res, writer: w}
				defer func() {
					if !grw.wroteBody {
						if res.Header().Get(bean.HeaderContentEncoding) == gzipScheme {
							res.Header().Del(bean.HeaderContentEncoding)
						}
						w.Reset(io.Discard)
					}
					_ = w.Close()
					pool.Put(w)
				}()
				c.SetResponse(grw)
			}
			return next(c)
		}
	}
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del(bean.HeaderContentLength) // Issue #444
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.Header().Get(bean.HeaderContentType) == "" {
		w.Header().Set(bean.HeaderContentType, http.DetectContentType(b))
	}
	w.wroteBody = true
	return w.writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	_ = w.writer.(*gzip.Writer).Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

func (w *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func gzipCompressPool(config GzipConfig) sync.Pool {
	return sync.Pool{
		New: func() interface{} {
			w, err := gzip.NewWriterLevel(io.Discard, config.Level)
			if err != nil {
				return err
			}
			return w
		},
	}
}
