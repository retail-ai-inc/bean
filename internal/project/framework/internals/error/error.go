/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package error

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	/**#bean*/
	"demo/framework/internals/stacktrace"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/stacktrace")**/)

var (
	ErrInternalServer      = errors.New("internal server error")
	ErrInvalidJsonResponse = errors.New("invalid JSON response")
	ErrContextExtraction   = errors.New("some data is missing in the context")
	ErrParamMissing        = errors.New("parameters are missing")
	ErrUpstreamTimeout     = errors.New("timeout from upstream server")
	ErrTimeout             = errors.New("timeout")
)

// APIError represents the error object of bean API error.
type APIError struct {
	HTTPStatusCode int
	GlobalErrCode  ErrorCode
	Err            error
	*stacktrace.Stack
}

// NewAPIError returns the proper error object from bean. You must provide `error` interface as 3rd parameter.
func NewAPIError(HTTPStatusCode int, globalErrCode ErrorCode, err error) *APIError {

	return &APIError{
		HTTPStatusCode: HTTPStatusCode,
		GlobalErrCode:  globalErrCode,
		Err:            err,
		Stack:          stacktrace.Callers(),
	}
}

// Format implements the `Formatter` interface
func (e *APIError) Format(s fmt.State, verb rune) {

	tmp := struct {
		HTTPStatusCode int
		GlobalErrCode  string
		Err            string
	}{
		e.HTTPStatusCode,
		string(e.GlobalErrCode),
		e.Error(),
	}

	out, err := json.Marshal(tmp)
	if err != nil {
		panic(err)
	}

	switch verb {
	case 'v':
		if s.Flag('+') {
			io.WriteString(s, string(out))
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, string(out))
	case 'q':
		fmt.Fprintf(s, "%q", out)
	}
}

func (e *APIError) Error() string {

	return e.Err.Error()
}
