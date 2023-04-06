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

		// Get retrieves data from the context.
		Get(key string) interface{}

		// Set saves data in the context.
		Set(key string, val interface{})

		// Bind binds the request body into provided type `i`. The default binder
		// does it based on Content-Type header.
		Bind(i interface{}) error

		// Validate validates provided `i`. It is usually called after `Context#Bind()`.
		// Validator must be registered using `Echo#Validator`.
		Validate(i interface{}) error

		// Render renders a template with data and sends a text/html response with status
		// code. Renderer must be registered using `Echo.Renderer`.
		Render(code int, name string, data interface{}) error

		// HTML sends an HTTP response with status code.
		HTML(code int, html string) error

		// HTMLBlob sends an HTTP blob response with status code.
		HTMLBlob(code int, b []byte) error

		// String sends a string response with status code.
		String(code int, s string) error

		// JSON sends a JSON response with status code.
		JSON(code int, i interface{}) error

		// Error invokes the registered HTTP error handler. Generally used by middleware.
		Error(err error)

		// Reset resets the context after request completes. It must be called along
		// with `Echo#AcquireContext()` and `Echo#ReleaseContext()`.
		// See `Echo#ServeHTTP()`
		Reset(r *http.Request, w http.ResponseWriter)
	}

	beanContext struct {
		request  *http.Request
		response http.ResponseWriter
	}

	HandlerFunc    func(c Context) error
	MiddlewareFunc func(HandlerFunc) HandlerFunc
)

var pool sync.Pool

func init() {
	pool.New = func() interface{} {
		return NewContext(nil, nil)
	}
}

func (c *beanContext) Request() *http.Request {
	return c.request
}

func (c *beanContext) SetRequest(r *http.Request) {
	c.request = r
}

func (c *beanContext) Response() http.ResponseWriter {
	return c.response
}

func (c *beanContext) Get(key string) interface{} {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) Set(key string, val interface{}) {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) Bind(i interface{}) error {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) Validate(i interface{}) error {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) Render(code int, name string, data interface{}) error {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) HTML(code int, html string) error {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) HTMLBlob(code int, b []byte) error {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) String(code int, s string) error {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) JSON(code int, i interface{}) error {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) Error(err error) {
	// TODO implement me
	panic("implement me")
}

func (c *beanContext) Reset(r *http.Request, w http.ResponseWriter) {
	c.request = r
	c.response = w
}

func NewContext(r *http.Request, w http.ResponseWriter) Context {
	return &beanContext{
		request:  r,
		response: w,
	}
}
