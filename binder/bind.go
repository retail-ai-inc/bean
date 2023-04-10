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

package binder

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/helpers"
	structure "github.com/retail-ai-inc/bean/struct"
)

const (
	HeaderContentType   = "Content-Type"
	MIMEApplicationJSON = "application/json"
)

// CustomBinder is an implementation of the Binder interface
// which only decodes JSON body and disallow unknow JSON fields.
type CustomBinder struct{}

// Bind implements the `Echo#Binder#Bind` function. It only decodes the JSON body for now.
// Extends it if you also need path or query params. Please reference the Echo#Binder to check how to do it.
func (cb *CustomBinder) Bind(i interface{}, c echo.Context) (err error) {
	return Bind(i, c.Request())
}

func Bind(i interface{}, req *http.Request) (err error) {
	if req.ContentLength == 0 {
		return
	}

	ctype := req.Header.Get(HeaderContentType)

	switch {

	case strings.HasPrefix(ctype, MIMEApplicationJSON) && (req.Method == http.MethodPost || req.Method == http.MethodPut || req.Method == http.MethodPatch):

		bodyBytes := bytes.NewBuffer(make([]byte, 0))
		reader := io.TeeReader(req.Body, bodyBytes)

		jc := json.NewDecoder(reader)
		jc.DisallowUnknownFields()

		if err = jc.Decode(i); err != nil {
			return err
		}

		// Restore the io.ReadCloser to its original state so that we can read c.Request().Body somewhere else.
		req.Body = io.NopCloser(bodyBytes)

		data, _ := helpers.PostDataStripTags(req, false)

		for key := range data {
			// We are ignoring the err. Here we are trying to match the actual JSON request parameter based on
			// JSON-RPC (case sensetive - https://jsonrpc.org/historical/json-rpc-1-1-alt.html#service-procedure-and-parameter-names).
			isTag, _ := structure.IsTagExist(key, "json", i)

			if !isTag {
				return errors.New("UnknownFieldsError")
			}
		}

	default:

		return
	}

	return
}
