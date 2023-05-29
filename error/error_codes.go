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

import "errors"

type ErrorCode string

const (
	API_SUCCESS ErrorCode = "000000"

	// API general error code
	PROBLEM_PARSING_JSON         ErrorCode = "100001"
	UNAUTHORIZED_ACCESS          ErrorCode = "100002"
	RESOURCE_NOT_FOUND           ErrorCode = "100003"
	INTERNAL_SERVER_ERROR        ErrorCode = "100004"
	REQUEST_ENTITY_TOO_LARGE     ErrorCode = "100005"
	METHOD_NOT_ALLOWED           ErrorCode = "100006"
	SERVICE_DOWN_FOR_MAINTENANCE ErrorCode = "100009"
	TOO_MANY_REQUESTS            ErrorCode = "100010"
	UNKNOWN_ERROR_CODE           ErrorCode = "100098"
	TIMEOUT                      ErrorCode = "100099"

	// API parameter error code
	API_DATA_VALIDATION_FAILED ErrorCode = "200001"
)

var (
	ErrInternalServer      = errors.New("internal server error")
	ErrInvalidJsonResponse = errors.New("invalid JSON response")
	ErrContextExtraction   = errors.New("some data is missing in the context")
	ErrParamMissing        = errors.New("parameters are missing")
	ErrUpstreamTimeout     = errors.New("timeout from upstream server")
	ErrTimeout             = errors.New("timeout")
)
