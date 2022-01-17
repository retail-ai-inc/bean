// The MIT License (MIT)

// Copyright (c) 2018 Foolin

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package echoview

import (
	"io"

	/**#bean*/
	"demo/framework/internals/goview"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/goview")**/

	"github.com/labstack/echo/v4"
)

const templateEngineKey = "foolin-goview-echoview"

// ViewEngine view engine for echo
type ViewEngine struct {
	*goview.ViewEngine
}

// New new view engine
func New(config goview.Config) *ViewEngine {
	return Wrap(goview.New(config))
}

// Wrap wrap view engine for goview.ViewEngine
func Wrap(engine *goview.ViewEngine) *ViewEngine {
	return &ViewEngine{
		ViewEngine: engine,
	}
}

// Default new default config view engine
func Default() *ViewEngine {
	return New(goview.DefaultConfig)
}

// Render render template for echo interface
func (e *ViewEngine) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return e.RenderWriter(w, name, data)
}

// Render html render for template
// You should use helper func `Middleware()` to set the supplied
// TemplateEngine and make `Render()` work validly.
func Render(ctx echo.Context, code int, name string, data interface{}) error {
	if val := ctx.Get(templateEngineKey); val != nil {
		if e, ok := val.(*ViewEngine); ok {
			return e.Render(ctx.Response().Writer, name, data, ctx)
		}
	}
	return ctx.Render(code, name, data)
}

// NewMiddleware echo middleware for func `echoview.Render()`
func NewMiddleware(config goview.Config) echo.MiddlewareFunc {
	return Middleware(New(config))
}

// Middleware echo middleware wrapper
func Middleware(e *ViewEngine) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(templateEngineKey, e)
			return next(c)
		}
	}
}
