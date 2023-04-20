package bean

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type (
	Context interface {
		context.Context

		// Request returns `*http.Request`.
		Request() *http.Request

		// SetRequest sets `*http.Request`.
		SetRequest(r *http.Request)

		// Response returns `http.ResponseWriter`.
		Response() ResponseWriter

		// SetResponse sets `http.ResponseWriter`.
		SetResponse(w ResponseWriter)

		// Keys returns all context keys set by Set.
		Keys() map[string]any

		// MustGet returns the value for the given key if it exists, otherwise it panics.
		MustGet(key string) any

		// Get returns the value for the given key string from the context.
		// If the value doesn't exist it returns (nil, false).
		Get(key string) (any, bool)

		// GetString returns the value associated with the key as a string.
		GetString(key string) (s string)

		// GetBool returns the value associated with the key as a boolean.
		GetBool(key string) (b bool)

		// GetInt returns the value associated with the key as an integer.
		GetInt(key string) (i int)

		// GetInt64 returns the value associated with the key as an integer.
		GetInt64(key string) (i64 int64)

		// GetUint returns the value associated with the key as an unsigned integer.
		GetUint(key string) (ui uint)

		// GetUint64 returns the value associated with the key as an unsigned integer.
		GetUint64(key string) (ui64 uint64)

		// GetFloat64 returns the value associated with the key as a float64.
		GetFloat64(key string) (f64 float64)

		// GetTime returns the value associated with the key as time.
		GetTime(key string) (t time.Time)

		// GetDuration returns the value associated with the key as a duration.
		GetDuration(key string) (d time.Duration)

		// GetStringSlice returns the value associated with the key as a slice of strings.
		GetStringSlice(key string) (ss []string)

		// GetStringMap returns the value associated with the key as a map of interfaces.
		GetStringMap(key string) (sm map[string]any)

		// GetStringMapString returns the value associated with the key as a map of strings.
		GetStringMapString(key string) (sms map[string]string)

		// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
		GetStringMapStringSlice(key string) (smss map[string][]string)

		// Set saves data in the context.
		Set(key string, val any)

		// Params returns all params set by AddParam.
		Params() [][2]string

		// Param returns URL parameter by name.
		Param(name string) string

		// AddParam adds param to context and.
		AddParam(name, value string)

		// Query returns the query param for the provided name.
		Query(name string) string

		// QueryParams returns the query parameters as `url.Values`.
		QueryParams() url.Values

		// Bind binds the request body into provided type `i`. The default binder
		// does it based on Content-Type header.
		Bind(i any) error

		// Validate validates provided `i`. It is usually called after `Context#Bind()`.
		// Validator must be registered using `Echo#Validator`.
		Validate(i any) error

		// Render renders a template with data and sends a text/html response with status
		// code. Renderer must be registered using `Echo.Renderer`.
		Render(code int, name string, data any) error

		// HTML sends an HTTP response with status code.
		HTML(code int, html string) error

		// HTMLBlob sends an HTTP blob response with status code.
		HTMLBlob(code int, b []byte) error

		// String sends a string response with status code.
		String(code int, s string) error

		// JSON sends a JSON response with status code. `charset` is an optional parameter.
		JSON(code int, i any, charset ...string) error

		// JSONP stands for JSON with Padding. Requesting a file from another domain can cause problems,
		// due to cross-domain policy. This function sends a JSONP response with status code.
		// It uses `callback` query param to construct the JSONP payload. `charset` is an optional parameter.
		JSONP(code int, i any, charset ...string) error

		// Error invokes the registered HTTP error handler. Generally used by middleware.
		Error(err error)

		// Reset resets the context after request completes. It must be called along
		// with `Echo#AcquireContext()` and `Echo#ReleaseContext()`.
		// See `Echo#ServeHTTP()`
		Reset(r *http.Request, w http.ResponseWriter)

		// Cookie returns the named cookie provided in the request.
		Cookie(name string) (*http.Cookie, error)

		// SetCookie adds a `Set-Cookie` header in HTTP response.
		SetCookie(cookie *http.Cookie)

		// Cookies returns the HTTP cookies sent with the request.
		Cookies() []*http.Cookie

		// RealIP returns the client's network address based on `X-Forwarded-For`
		// or `X-Real-IP` request header.
		// The behavior can be configured using `Echo#IPExtractor`.
		RealIP() string

		// FormValue returns the form field value for the provided name.
		FormValue(name string) string

		// FormParams returns the form parameters as `url.Values`.
		FormParams() (url.Values, error)

		// FormFile returns the multipart form file for the provided name.
		FormFile(name string) (*multipart.FileHeader, error)

		// MultipartForm returns the multipart form.
		MultipartForm() (*multipart.Form, error)
	}

	Binder interface {
		Bind(i any) error
	}

	// Validator is the interface that wraps the Validate function.
	Validator interface {
		Validate(i any) error
	}

	beanContext struct {
		request   *http.Request
		response  ResponseWriter
		mu        sync.RWMutex
		keys      map[string]any
		binder    Binder
		validator Validator
		params    [][2]string
		query     url.Values

		bean *Bean
	}

	HandlerFunc    func(c Context) error
	MiddlewareFunc func(HandlerFunc) HandlerFunc
)

const (
	defaultIndent = "  "
	defaultMemory = 32 << 20 // 32 MB
)

var (
	// beanContext implement the Context interface
	_ Context = (*beanContext)(nil)
)

const (
	jsonCType  = "application/json"
	jsonpCType = "application/javascript"
)

func (bc *beanContext) Request() *http.Request {
	return bc.request
}

func (bc *beanContext) SetRequest(r *http.Request) {
	bc.request = r
}

func (bc *beanContext) Response() ResponseWriter {
	return bc.response
}

func (bc *beanContext) SetResponse(w ResponseWriter) {
	bc.response = w
}

func (bc *beanContext) Keys() map[string]any {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.keys
}

// MustGet returns the value for the given key if it exists, otherwise it panics.
func (bc *beanContext) MustGet(key string) any {
	val, e := bc.Get(key)
	if !e {
		panic(fmt.Sprintf("beanContext: %q not exist", key))
	}
	return val
}

// Get returns the value for the given key string from the context.
// If the value doesn't exist it returns (nil, false).
func (bc *beanContext) Get(key string) (value any, e bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	value, e = bc.keys[key]
	return
}

// GetString returns the value associated with the key as a string.
func (bc *beanContext) GetString(key string) (s string) {
	if val, ok := bc.Get(key); ok && val != nil {
		s, _ = val.(string)
	}
	return
}

// GetBool returns the value associated with the key as a boolean.
func (bc *beanContext) GetBool(key string) (b bool) {
	if val, ok := bc.Get(key); ok && val != nil {
		b, _ = val.(bool)
	}
	return
}

// GetInt returns the value associated with the key as an integer.
func (bc *beanContext) GetInt(key string) (i int) {
	if val, ok := bc.Get(key); ok && val != nil {
		i, _ = val.(int)
	}
	return
}

// GetInt64 returns the value associated with the key as an integer.
func (bc *beanContext) GetInt64(key string) (i64 int64) {
	if val, ok := bc.Get(key); ok && val != nil {
		i64, _ = val.(int64)
	}
	return
}

// GetUint returns the value associated with the key as an unsigned integer.
func (bc *beanContext) GetUint(key string) (ui uint) {
	if val, ok := bc.Get(key); ok && val != nil {
		ui, _ = val.(uint)
	}
	return
}

// GetUint64 returns the value associated with the key as an unsigned integer.
func (bc *beanContext) GetUint64(key string) (ui64 uint64) {
	if val, ok := bc.Get(key); ok && val != nil {
		ui64, _ = val.(uint64)
	}
	return
}

// GetFloat64 returns the value associated with the key as a float64.
func (bc *beanContext) GetFloat64(key string) (f64 float64) {
	if val, ok := bc.Get(key); ok && val != nil {
		f64, _ = val.(float64)
	}
	return
}

// GetTime returns the value associated with the key as time.
func (bc *beanContext) GetTime(key string) (t time.Time) {
	if val, ok := bc.Get(key); ok && val != nil {
		t, _ = val.(time.Time)
	}
	return
}

// GetDuration returns the value associated with the key as a duration.
func (bc *beanContext) GetDuration(key string) (d time.Duration) {
	if val, ok := bc.Get(key); ok && val != nil {
		d, _ = val.(time.Duration)
	}
	return
}

// GetStringSlice returns the value associated with the key as a slice of strings.
func (bc *beanContext) GetStringSlice(key string) (ss []string) {
	if val, ok := bc.Get(key); ok && val != nil {
		ss, _ = val.([]string)
	}
	return
}

// GetStringMap returns the value associated with the key as a map of interfaces.
func (bc *beanContext) GetStringMap(key string) (sm map[string]any) {
	if val, ok := bc.Get(key); ok && val != nil {
		sm, _ = val.(map[string]any)
	}
	return
}

// GetStringMapString returns the value associated with the key as a map of strings.
func (bc *beanContext) GetStringMapString(key string) (sms map[string]string) {
	if val, ok := bc.Get(key); ok && val != nil {
		sms, _ = val.(map[string]string)
	}
	return
}

// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
func (bc *beanContext) GetStringMapStringSlice(key string) (smss map[string][]string) {
	if val, ok := bc.Get(key); ok && val != nil {
		smss, _ = val.(map[string][]string)
	}
	return
}

// get returns the value associated with the key as a T. for example get[string](bc,key)
func get[T any](bc *beanContext, key string) (value T) {
	if val, ok := bc.Get(key); ok && val != nil {
		value, _ = val.(T)
	}
	return
}

// Set is saving a new key-value pair exclusively for this context.
// It also initializes `bc.keys` if it was not initialized previously.
func (bc *beanContext) Set(key string, value any) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.keys == nil {
		bc.keys = make(map[string]any)
	}

	bc.keys[key] = value
}

// Params returns all params set by AddParam.
func (bc *beanContext) Params() [][2]string {
	return bc.params
}

// Param returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (bc *beanContext) Param(name string) string {
	for _, data := range bc.params {
		if data[0] == name {
			return data[1]
		}
	}
	return ""
}

func (bc *beanContext) AddParam(name, value string) {
	bc.params = append(bc.params, [2]string{name, value})
}

func (bc *beanContext) Query(name string) string {
	return bc.QueryParams().Get(name)
}

func (bc *beanContext) QueryParams() url.Values {
	if bc.query == nil {
		bc.query = bc.request.URL.Query()
	}
	return bc.query
}

func (bc *beanContext) Bind(i any) error {
	return bc.binder.Bind(i)
}

func (bc *beanContext) SetBinder(binder Binder) {
	bc.binder = binder
}

func (bc *beanContext) Validate(i any) error {
	if bc.validator != nil {
		return bc.validator.Validate(i)
	}
	return nil
}

func (bc *beanContext) Render(code int, name string, data any) error {
	// TODO implement me
	panic("implement me")
}

func (bc *beanContext) HTML(code int, html string) error {
	// TODO implement me
	panic("implement me")
}

func (bc *beanContext) HTMLBlob(code int, b []byte) error {
	// TODO implement me
	panic("implement me")
}

func (bc *beanContext) String(code int, s string) error {
	return bc.Blob(code, MIMETextPlainCharsetUTF8, []byte(s))
}

func (bc *beanContext) Blob(code int, contentType string, b []byte) (err error) {
	bc.writeContentType(contentType)
	bc.response.WriteHeader(code)
	_, err = bc.response.Write(b)
	return
}

func (bc *beanContext) JSON(code int, i any, charset ...string) error {
	return bc.json(code, i, bc.indentFromQueryParam(), charset...)
}

func (bc *beanContext) JSONP(code int, i any, charset ...string) (err error) {
	return bc.jsonp(code, i, bc.indentFromQueryParam(), charset...)
}

func (bc *beanContext) indentFromQueryParam() (indent string) {
	if _, pretty := bc.QueryParams()["pretty"]; pretty {
		indent = defaultIndent
	}
	return indent
}

func (bc *beanContext) json(code int, i any, indent string, charset ...string) error {
	if len(charset) > 0 {
		bc.writeContentType(jsonCType + ";" + charset[0])
	} else {
		bc.writeContentType(jsonCType + ";" + "charset=utf-8")
	}
	bc.response.WriteHeader(code)
	enc := json.NewEncoder(bc.response)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	return enc.Encode(i)
}

func (bc *beanContext) jsonp(code int, i any, indent string, charset ...string) (err error) {
	if len(charset) > 0 {
		bc.writeContentType(jsonpCType + ";" + charset[0])
	} else {
		bc.writeContentType(jsonpCType + ";" + "charset=utf-8")
	}
	bc.response.WriteHeader(code)

	callback := bc.Query("callback")
	if callback == "" {
		enc := json.NewEncoder(bc.response)
		if indent != "" {
			enc.SetIndent("", indent)
		}
		return enc.Encode(i)
	} else {
		if _, err = bc.response.Write([]byte(callback + "(")); err != nil {
			return
		}
		enc := json.NewEncoder(bc.response)
		if indent != "" {
			enc.SetIndent("", indent)
		}
		if err = enc.Encode(i); err != nil {
			return
		}
		if _, err = bc.response.Write([]byte(");")); err != nil {
			return
		}
		return
	}
}

func (bc *beanContext) writeContentType(value string) {
	header := bc.response.Header()
	if header.Get(HeaderContentType) == "" {
		header.Set(HeaderContentType, value)
	}
}

func (bc *beanContext) Error(err error) {
	// TODO implement me
	panic("implement me")
}

func (bc *beanContext) Reset(r *http.Request, w http.ResponseWriter) {
	bc.request = r
	bc.response.Reset(w)
	bc.keys = nil
	bc.binder = nil
	bc.validator = nil
	bc.params = nil
	bc.query = nil
}

func (bc *beanContext) Cookie(name string) (*http.Cookie, error) {
	return bc.request.Cookie(name)
}

func (bc *beanContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(bc.Response(), cookie)
}

func (bc *beanContext) Cookies() []*http.Cookie {
	return bc.request.Cookies()
}

func (bc *beanContext) RealIP() string {
	// Fall back to legacy behavior
	if ip := bc.request.Header.Get(HeaderXForwardedFor); ip != "" {
		i := strings.IndexAny(ip, ",")
		if i > 0 {
			xffip := strings.TrimSpace(ip[:i])
			xffip = strings.TrimPrefix(xffip, "[")
			xffip = strings.TrimSuffix(xffip, "]")
			return xffip
		}
		return ip
	}
	if ip := bc.request.Header.Get(HeaderXRealIP); ip != "" {
		ip = strings.TrimPrefix(ip, "[")
		ip = strings.TrimSuffix(ip, "]")
		return ip
	}
	ra, _, _ := net.SplitHostPort(bc.request.RemoteAddr)
	return ra
}

// Deadline returns that there is no deadline (ok==false) when c.Request has no Context.
func (bc *beanContext) Deadline() (deadline time.Time, ok bool) {
	if bc.request == nil {
		return
	}
	return bc.request.Context().Deadline()
}

// Done returns nil (chan which will wait forever) when c.Request has no Context.
func (bc *beanContext) Done() <-chan struct{} {
	if bc.request == nil {
		return nil
	}
	return bc.request.Context().Done()
}

// Err returns nil when c.Request has no Context.
func (bc *beanContext) Err() error {
	if bc.request == nil {
		return nil
	}
	return bc.request.Context().Err()
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
func (bc *beanContext) Value(key any) any {
	if key == 0 {
		return bc.request
	}

	if keyAsString, ok := key.(string); ok {
		if val, exists := bc.Get(keyAsString); exists {
			return val
		}
	}
	if bc.request == nil {
		return nil
	}
	return bc.request.Context().Value(key)
}

func (bc *beanContext) FormValue(name string) string {
	return bc.request.FormValue(name)
}

func (bc *beanContext) FormParams() (url.Values, error) {
	if strings.HasPrefix(bc.request.Header.Get(HeaderContentType), MIMEMultipartForm) {
		if err := bc.request.ParseMultipartForm(defaultMemory); err != nil {
			return nil, err
		}
	} else {
		if err := bc.request.ParseForm(); err != nil {
			return nil, err
		}
	}
	return bc.request.Form, nil
}

func (bc *beanContext) FormFile(name string) (*multipart.FileHeader, error) {
	f, fh, err := bc.request.FormFile(name)
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}
	return fh, nil
}

func (bc *beanContext) MultipartForm() (*multipart.Form, error) {
	err := bc.request.ParseMultipartForm(defaultMemory)
	return bc.request.MultipartForm, err
}

func (bc *beanContext) SentryCaptureException(err error) {
	bc.bean.SentryCaptureException(bc, err)
}

func (bc *beanContext) SentryCaptureMessage(msg string) {
	bc.bean.SentryCaptureMessage(bc, msg)
}
