// Copyright The RAI Inc.
// The RAI Authors
package error

import (
	"fmt"

	"github.com/retail-ai-inc/bean/stacktrace"
)

// APIError represents the error object of {{ .PkgPath }} API error.
type APIError struct {
	HTTPStatusCode int
	GlobalErrCode  ErrorCode
	Err            error
	*stacktrace.Stack
}

// NewAPIError returns the proper error object from {{ .PkgPath }}. You must provide `error` interface as 3rd parameter.
func NewAPIError(HTTPStatusCode int, globalErrCode ErrorCode, err error) *APIError {
	return &APIError{
		HTTPStatusCode: HTTPStatusCode,
		GlobalErrCode:  globalErrCode,
		Err:            err,
		Stack:          stacktrace.Callers(),
	}
}

// This function need to be call explicitly because the APIError embedded the *stacktrace.Stack which already implemented the Format()
// function and treat it as a formatter. Example: fmt.Println(e.String())
func (e *APIError) String() string {
	return fmt.Sprintf(`{"HTTPStatusCode":%d, "GlobalErrCode":%s, "Err":%s}`, e.HTTPStatusCode, e.GlobalErrCode, e.Err)
}

// This function need to be call explicitly because the APIError embedded the *stacktrace.Stack which already implemented the Format()
// function and treat it as a formatter. Example: fmt.Println(e.String())
func (e *APIError) Error() string {
	return e.Err.Error()
}
