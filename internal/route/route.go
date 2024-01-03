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

package route

import (
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/internal/url"
)

// Route contains all the registered routes name, URI path, request method and URI path template.
type Route struct {
	Method      string
	Path        string
	Name        string
	PathSegment url.Path
}

// Routes is a global variable to hold all necessary route information.
var Routes = []Route{}

func Init(e *echo.Echo) {
	Routes = make([]Route, len(e.Routes()))

	for i, r := range e.Routes() {
		Routes[i].Method = r.Method
		Routes[i].Path = r.Path
		Routes[i].Name = r.Name
		Routes[i].PathSegment = url.New(r.Path)
	}
}
