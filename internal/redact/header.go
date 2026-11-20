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

// Package redact masks sensitive values before they are written to logs.
package redact

import (
	"net/http"
	"strings"
)

// Redacted replaces the value of a sensitive header in logs.
const Redacted = "[REDACTED]"

// sensitiveHeaders are canonical header names whose values are always masked.
var sensitiveHeaders = map[string]struct{}{
	"Authorization":       {},
	"Proxy-Authorization": {},
	"Cookie":              {},
	"Set-Cookie":          {},
}

// sensitiveFragments mask any header whose lower-cased name contains one of
// them, e.g. X-Api-Key, X-Auth-Token, X-Client-Secret, X-Session-Id.
var sensitiveFragments = []string{
	"token",
	"secret",
	"password",
	"passwd",
	"api-key",
	"apikey",
	"session",
	"signature",
	"credential",
}

// IsSensitiveHeader reports whether the value of the header name must not be logged in clear text.
func IsSensitiveHeader(name string) bool {
	if _, ok := sensitiveHeaders[http.CanonicalHeaderKey(name)]; ok {
		return true
	}

	lower := strings.ToLower(name)
	for _, f := range sensitiveFragments {
		if strings.Contains(lower, f) {
			return true
		}
	}

	return false
}

// HeaderValue returns value as it is safe to log for the header name.
// Sensitive values are replaced with Redacted; for Authorization-style headers
// the auth scheme (e.g. "Bearer") is kept so the log is still useful.
func HeaderValue(name, value string) string {
	if !IsSensitiveHeader(name) {
		return value
	}

	switch http.CanonicalHeaderKey(name) {
	case "Authorization", "Proxy-Authorization":
		if scheme, _, ok := strings.Cut(strings.TrimSpace(value), " "); ok && scheme != "" {
			return scheme + " " + Redacted
		}
	}

	return Redacted
}

// Headers collects the non-empty values of the given header names from h,
// keyed by the configured name, with sensitive values redacted.
func Headers(h http.Header, names []string) map[string]any {
	out := make(map[string]any, len(names))
	for _, name := range names {
		if v := h.Get(name); v != "" {
			out[name] = HeaderValue(name, v)
		}
	}
	return out
}
