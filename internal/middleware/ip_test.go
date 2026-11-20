package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNewIPExtractorEmptyKeepsDirectIP(t *testing.T) {
	extractor, err := NewIPExtractor(nil)
	if err != nil {
		t.Fatalf("NewIPExtractor() error = %v", err)
	}
	if extractor != nil {
		t.Fatal("expected no extractor for an empty trust range")
	}

	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set(echo.HeaderXForwardedFor, "198.51.100.20")
	c := e.NewContext(req, httptest.NewRecorder())

	if got := c.RealIP(); got != "203.0.113.10" {
		t.Fatalf("RealIP() = %q, want direct peer", got)
	}
}

func TestNewIPExtractorUsesConfiguredTrustIPRange(t *testing.T) {
	extractor, err := NewIPExtractor([]string{" 198.51.100.0/24 "})
	if err != nil {
		t.Fatalf("NewIPExtractor() error = %v", err)
	}

	e := echo.New()
	e.IPExtractor = extractor
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set(echo.HeaderXForwardedFor, "203.0.113.20")
	c := e.NewContext(req, httptest.NewRecorder())

	if got := c.RealIP(); got != "203.0.113.20" {
		t.Fatalf("RealIP() = %q, want forwarded client", got)
	}
}

func TestNewIPExtractorAcceptsSingleIP(t *testing.T) {
	extractor, err := NewIPExtractor([]string{"198.51.100.10"})
	if err != nil {
		t.Fatalf("NewIPExtractor() error = %v", err)
	}

	e := echo.New()
	e.IPExtractor = extractor
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set(echo.HeaderXForwardedFor, "203.0.113.20")
	c := e.NewContext(req, httptest.NewRecorder())

	if got := c.RealIP(); got != "203.0.113.20" {
		t.Fatalf("RealIP() = %q, want forwarded client", got)
	}
}

func TestNewIPExtractorTrustsLoopbackPeer(t *testing.T) {
	extractor, err := NewIPExtractor([]string{"34.102.207.206/32"})
	if err != nil {
		t.Fatalf("NewIPExtractor() error = %v", err)
	}

	e := echo.New()
	e.IPExtractor = extractor
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.6:12345"
	req.Header.Set(echo.HeaderXForwardedFor, "221.214.179.58,34.102.207.206")
	c := e.NewContext(req, httptest.NewRecorder())

	if got := c.RealIP(); got != "221.214.179.58" {
		t.Fatalf("RealIP() = %q, want client from X-Forwarded-For", got)
	}
}

func TestNewIPExtractorIgnoresPrivatePeerUnlessConfigured(t *testing.T) {
	extractor, err := NewIPExtractor([]string{"198.51.100.0/24"})
	if err != nil {
		t.Fatalf("NewIPExtractor() error = %v", err)
	}

	e := echo.New()
	e.IPExtractor = extractor
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.10:12345"
	req.Header.Set(echo.HeaderXForwardedFor, "203.0.113.20")
	c := e.NewContext(req, httptest.NewRecorder())

	if got := c.RealIP(); got != "10.0.0.10" {
		t.Fatalf("RealIP() = %q, want direct private peer", got)
	}
}

func TestNewIPExtractorRejectsInvalidCIDR(t *testing.T) {
	_, err := NewIPExtractor([]string{"10.0.0.0/8", "not-a-cidr"})
	if err == nil || !strings.Contains(err.Error(), `http.trustIPRange contains invalid IP or CIDR "not-a-cidr"`) {
		t.Fatalf("error = %v", err)
	}
}
