package bean

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A panicking request must be counted in the Prometheus metrics as 500.
func TestNewEcho_PanicIsCountedInMetrics(t *testing.T) {
	// Use a fresh registry so the test does not depend on / pollute the global one.
	reg := prometheus.NewRegistry()
	origRegisterer, origGatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = reg, reg
	t.Cleanup(func() { prometheus.DefaultRegisterer, prometheus.DefaultGatherer = origRegisterer, origGatherer })

	conf := validTestConfig()
	conf.Prometheus.On = true
	useConfig(t, conf)

	e, closeEcho, err := NewEcho()
	require.NoError(t, err)
	t.Cleanup(func() { _ = closeEcho() })

	e.GET("/panic", func(c *echo.Context) error { panic("boom") })
	e.GET("/ok", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for _, path := range []string{"/panic", "/ok"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if path == "/panic" {
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	assert.True(t, hasMetric(rec.Body.String(), "echo_requests_total", `code="500"`, `url="/panic"`), rec.Body.String())
	assert.True(t, hasMetric(rec.Body.String(), "echo_requests_total", `code="200"`, `url="/ok"`), rec.Body.String())
}

func hasMetric(body, name string, labels ...string) bool {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, name+"{") {
			continue
		}
		ok := true
		for _, l := range labels {
			if !strings.Contains(line, l) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
