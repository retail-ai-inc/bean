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

package error

import (
	"fmt"

	"github.com/retail-ai-inc/bean/v2/stacktrace"
)

// APIError represents the error object of {{ .PkgPath }} API error.
type APIError struct {
	HTTPStatusCode int
	GlobalErrCode  ErrorCode
	Err            error
	Ignorable      bool // An extra option to control the behaviour. (example: push to some error tracker or not)
	*stacktrace.Stack
}

// NewAPIError returns the proper error object from {{ .PkgPath }}. You must provide `error` interface as 3rd parameter.
func NewAPIError(HTTPStatusCode int, globalErrCode ErrorCode, err error) *APIError {
	return &APIError{
		HTTPStatusCode: HTTPStatusCode,
		GlobalErrCode:  globalErrCode,
		Err:            err,
		Ignorable:      false,
		Stack:          stacktrace.Callers(),
	}
}

// NewIgnorableAPIError returns the proper error object from {{ .PkgPath }},
// the error tracker like sentry will not push this type of error online. You must provide `error` interface as 3rd parameter.
func NewIgnorableAPIError(HTTPStatusCode int, globalErrCode ErrorCode, err error) *APIError {
	e := NewAPIError(HTTPStatusCode, globalErrCode, err)
	e.Ignorable = true
	return e
}

// This function need to be call explicitly because the APIError embedded the *stacktrace.Stack which already implemented the Format()
// function and treat it as a formatter. Example: fmt.Println(e.String())
func (e *APIError) String() string {
	return fmt.Sprintf(`{"HTTPStatusCode":%d, "GlobalErrCode":%s, "Err":%s}`, e.HTTPStatusCode, e.GlobalErrCode, e.Err)
}

// This function need to be call explicitly because the APIError embedded the *stacktrace.Stack which already implemented the Format()
// function and treat it as a formatter. Example: fmt.Println(e.String())
func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "No error detail available"
}
