package bean

import (
	"net/http"
	"sync"
)

type (
	Context interface {
		// Request returns `*http.Request`.
		Request() *http.Request

		// SetRequest sets `*http.Request`.
		SetRequest(r *http.Request)

		// Response returns `http.ResponseWriter`.
		Response() http.ResponseWriter

		// Get returns the value for the given key string from the context.
		// If the value doesn't exist it returns (nil, false).
		Get(key string) (any, bool)

		// Set saves data in the context.
		Set(key string, val any)

		Keys() map[string]any

		// Bind binds the request body into provided type `i`. The default binder
		// does it based on Content-Type header.
		Bind(i any, _ Context) error

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

		// JSON sends a JSON response with status code.
		JSON(code int, i any) error

		// Error invokes the registered HTTP error handler. Generally used by middleware.
		Error(err error)

		// Reset resets the context after request completes. It must be called along
		// with `Echo#AcquireContext()` and `Echo#ReleaseContext()`.
		// See `Echo#ServeHTTP()`
		Reset(r *http.Request, w http.ResponseWriter)
	}

	Binder interface {
		Bind(i interface{}) error
	}
	
	beanContext struct {
		request  *http.Request
		response http.ResponseWriter
		mu       sync.RWMutex
		keys     map[string]any
		binder   Binder
	}

	HandlerFunc    func(c Context) error
	MiddlewareFunc func(HandlerFunc) HandlerFunc
)

// beanContext must implement the Context interface
var _ Context = (*beanContext)(nil)

var (
	pool sync.Pool
)

func init() {
	pool.New = func() any {
		return NewContext(nil, nil)
	}
}

func NewContext(r *http.Request, w http.ResponseWriter) *beanContext {
	return &beanContext{
		request:  r,
		response: w,
	}
}

func (bc *beanContext) Request() *http.Request {
	return bc.request
}

func (bc *beanContext) SetRequest(r *http.Request) {
	bc.request = r
}

func (bc *beanContext) Response() http.ResponseWriter {
	return bc.response
}

func (bc *beanContext) Keys() map[string]any {
	return bc.keys
}

// Get returns the value for the given key string from the context.
// If the value doesn't exist it returns (nil, false).
func (bc *beanContext) Get(key string) (value any, e bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	value, e = bc.keys[key]
	return
}

// Set is saving a new key-value pair exclusively for this context.
// It also initializes `bc.keys` if it was not initialized previously.
func (bc *beanContext) Set(key string, val any) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.keys == nil {
		bc.keys = make(map[string]any)
	}

	bc.keys[key] = val
}

func (bc *beanContext) Bind(i any, _ Context) error {
	return bc.binder.Bind(i)
}

func (bc *beanContext) SetBinder(binder Binder) {
	bc.binder = binder
}

func (bc *beanContext) Validate(i any) error {
	// TODO implement me
	panic("implement me")
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
	// TODO implement me
	panic("implement me")
}

func (bc *beanContext) JSON(code int, i any) error {
	// TODO implement me
	panic("implement me")
}

func (bc *beanContext) Error(err error) {
	// TODO implement me
	panic("implement me")
}

func (bc *beanContext) Reset(r *http.Request, w http.ResponseWriter) {
	bc.request = r
	bc.response = w
}
