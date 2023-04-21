package middleware

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean"
	"github.com/stretchr/testify/assert"
)

func TestGzip(t *testing.T) {
	b := bean.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	bc := b.NewContext(req, rec)

	// Skip if no Accept-Encoding header
	h := Gzip()(func(c bean.Context) error {
		_, _ = c.Response().Write([]byte("test")) // For Content-Type sniffing
		return nil
	})
	_ = h(bc)

	assert.Equal(t, "test", rec.Body.String())

	// Gzip
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)
	rec = httptest.NewRecorder()
	bc = b.NewContext(req, rec)
	_ = h(bc)
	assert.Equal(t, gzipScheme, rec.Header().Get(bean.HeaderContentEncoding))
	assert.Contains(t, rec.Header().Get(bean.HeaderContentType), bean.MIMETextPlain)
	r, err := gzip.NewReader(rec.Body)
	if assert.NoError(t, err) {
		buf := new(bytes.Buffer)
		defer func(r *gzip.Reader) {
			_ = r.Close()
		}(r)
		_, _ = buf.ReadFrom(r)
		assert.Equal(t, "test", buf.String())
	}

	chunkBuf := make([]byte, 5)

	// Gzip chunked
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)
	rec = httptest.NewRecorder()

	bc = b.NewContext(req, rec)
	_ = Gzip()(func(c bean.Context) error {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Transfer-Encoding", "chunked")

		// Write and flush the first part of the data
		_, _ = c.Response().Write([]byte("test\n"))
		c.Response().Flush()

		// Read the first part of the data
		assert.True(t, rec.Flushed)
		assert.Equal(t, gzipScheme, rec.Header().Get(bean.HeaderContentEncoding))
		_ = r.Reset(rec.Body)

		_, err = io.ReadFull(r, chunkBuf)
		assert.NoError(t, err)
		assert.Equal(t, "test\n", string(chunkBuf))

		// Write and flush the second part of the data
		_, _ = c.Response().Write([]byte("test\n"))
		c.Response().Flush()

		_, err = io.ReadFull(r, chunkBuf)
		assert.NoError(t, err)
		assert.Equal(t, "test\n", string(chunkBuf))

		// Write the final part of the data and return
		_, _ = c.Response().Write([]byte("test"))
		return nil
	})(bc)

	buf := new(bytes.Buffer)
	defer func(r *gzip.Reader) {
		_ = r.Close()
	}(r)
	_, _ = buf.ReadFrom(r)
	assert.Equal(t, "test", buf.String())
}

func TestGzipNoContent(t *testing.T) {
	b := bean.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)
	rec := httptest.NewRecorder()
	bc := b.NewContext(req, rec)
	h := Gzip()(func(c bean.Context) error {
		c.Response().WriteHeader(http.StatusNoContent)
		return nil
	})
	if assert.NoError(t, h(bc)) {
		assert.Empty(t, rec.Header().Get(bean.HeaderContentEncoding))
		assert.Empty(t, rec.Header().Get(bean.HeaderContentType))
		assert.Equal(t, 0, len(rec.Body.Bytes()))
	}
}

func TestGzipEmpty(t *testing.T) {
	b := bean.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)
	rec := httptest.NewRecorder()
	bc := b.NewContext(req, rec)
	h := Gzip()(func(c bean.Context) error {
		return c.String(http.StatusOK, "")
	})
	if assert.NoError(t, h(bc)) {
		assert.Equal(t, gzipScheme, rec.Header().Get(bean.HeaderContentEncoding))
		assert.Equal(t, "text/plain; charset=UTF-8", rec.Header().Get(bean.HeaderContentType))
		r, err := gzip.NewReader(rec.Body)
		if assert.NoError(t, err) {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			assert.Equal(t, "", buf.String())
		}
	}
}

func TestGzipErrorReturned(t *testing.T) {
	e := echo.New()
	e.Use(bean.WrapEchoMiddleware(Gzip()))
	e.GET("/", bean.WrapEchoHandler(func(c bean.Context) error {
		return errors.New(http.StatusText(http.StatusNotFound))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Empty(t, rec.Header().Get(bean.HeaderContentEncoding))
}

func TestGzipErrorReturnedInvalidConfig(t *testing.T) {
	e := echo.New()
	// Invalid level
	e.Use(bean.WrapEchoMiddleware(GzipWithConfig(GzipConfig{Level: 12})))
	e.GET("/", bean.WrapEchoHandler(func(c bean.Context) error {
		_, _ = c.Response().Write([]byte("test"))
		return nil
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "gzip")
}

// Issue #806
func TestGzipWithStatic(t *testing.T) {
	e := echo.New()
	e.Use(bean.WrapEchoMiddleware(Gzip()))
	e.Static("/test", "../docs/static")
	req := httptest.NewRequest(http.MethodGet, "/test/service_repository_pattern.png", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	// Data is written out in chunks when Content-Length == "", so only
	// validate the content length if it's not set.
	if cl := rec.Header().Get("Content-Length"); cl != "" {
		assert.Equal(t, cl, rec.Body.Len())
	}
	r, err := gzip.NewReader(rec.Body)
	if assert.NoError(t, err) {
		defer func(r *gzip.Reader) {
			_ = r.Close()
		}(r)
		want, err := os.ReadFile("../docs/static/service_repository_pattern.png")
		if assert.NoError(t, err) {
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(r)
			assert.Equal(t, want, buf.Bytes())
		}
	}
}

func BenchmarkGzip(b *testing.B) {
	e := bean.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(bean.HeaderAcceptEncoding, gzipScheme)

	h := Gzip()(func(c bean.Context) error {
		_, _ = c.Response().Write([]byte("test")) // For Content-Type sniffing
		return nil
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Gzip
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		_ = h(c)
	}
}
