package redact

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeaderValue(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{"bearer token keeps scheme", "Authorization", "Bearer abc.def.ghi", "Bearer " + Mask},
		{"lower-case name", "authorization", "Basic dXNlcjpwYXNz", "Basic " + Mask},
		{"proxy authorization", "Proxy-Authorization", "Bearer xyz", "Bearer " + Mask},
		{"authorization without scheme", "Authorization", "rawsecret", Mask},
		{"cookie", "Cookie", "sid=abc; theme=dark", Mask},
		{"set-cookie", "Set-Cookie", "sid=abc; HttpOnly", Mask},
		{"api key", "X-Api-Key", "k-123", Mask},
		{"apikey without dash", "X-APIKEY", "k-123", Mask},
		{"auth token", "X-Auth-Token", "t-123", Mask},
		{"client secret", "X-Client-Secret", "s-123", Mask},
		{"session id", "X-Session-Id", "s-123", Mask},
		{"request id is kept", "X-Request-Id", "req-1", "req-1"},
		{"content type is kept", "Content-Type", "application/json", "application/json"},
		{"user agent is kept", "User-Agent", "curl/8.0", "curl/8.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HeaderValue(tt.header, tt.value))
		})
	}
}

func TestHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer secret-token")
	h.Set("Cookie", "sid=abc")
	h.Set("X-Request-Id", "req-1")
	h.Set("Content-Type", "application/json")

	got := Headers(h, []string{"Authorization", "cookie", "X-Request-Id", "Content-Type", "X-Missing"})

	assert.Equal(t, map[string]any{
		"Authorization": "Bearer " + Mask,
		"cookie":        Mask,
		"X-Request-Id":  "req-1",
		"Content-Type":  "application/json",
	}, got)
}

func TestHeadersEmpty(t *testing.T) {
	assert.Empty(t, Headers(http.Header{}, nil))
	assert.Empty(t, Headers(http.Header{"Authorization": {"Bearer x"}}, nil))
}
