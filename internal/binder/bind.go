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
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
	berror "github.com/retail-ai-inc/bean/v3/error"
)

// CustomBinder is an implementation of the Binder interface
// which only decodes JSON body and disallow unknow JSON fields.
type CustomBinder struct{}

// Bind implements the `Echo#Binder#Bind` function. It only decodes the JSON body for now.
// Extends it if you also need path or query params. Please reference the Echo#Binder to check how to do it.
//
// The body must be a single JSON value whose fields all exist in i: unknown fields are rejected, and the top level
// field names must match exactly (encoding/json matches names case-insensitively). An invalid body returns a 400
// `*berror.APIError` with the `PROBLEM_PARSING_JSON` code. The body can be read again after Bind.
func (cb *CustomBinder) Bind(c *echo.Context, i interface{}) (err error) {

	req := c.Request()
	if req.ContentLength == 0 {
		return
	}

	ctype := req.Header.Get(echo.HeaderContentType)

	switch {

	case strings.HasPrefix(ctype, echo.MIMEApplicationJSON) && (req.Method == http.MethodPost || req.Method == http.MethodPut || req.Method == http.MethodPatch):

		body, err := io.ReadAll(req.Body)
		if err != nil {
			return err // e.g. the body limit error, which carries its own status
		}
		// Restore the body so that it can be read somewhere else.
		req.Body = io.NopCloser(bytes.NewReader(body))

		if err := decodeJSON(body, i); err != nil {
			return berror.NewAPIError(http.StatusBadRequest, berror.PROBLEM_PARSING_JSON,
				fmt.Errorf("invalid JSON request body: %w", err))
		}

	default:

		return
	}

	return
}

func decodeJSON(body []byte, i interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(i); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON value")
	}

	// encoding/json matches the field names case-insensitively: check that the top level names match exactly
	// (JSON-RPC, https://jsonrpc.org/historical/json-rpc-1-1-alt.html#service-procedure-and-parameter-names).
	names, ok := fieldNames(reflect.TypeOf(i))
	if !ok {
		return nil // not a struct: nothing to check
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return nil // not a JSON object (e.g. null)
	}
	for key := range object {
		if _, ok := names[key]; !ok {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	return nil
}

var fieldNamesCache sync.Map // map[reflect.Type]map[string]struct{}

// fieldNames returns the JSON names of the fields of the struct t (or pointer to struct) as encoding/json decodes
// them, including the fields of embedded structs. ok is false if t is not a struct.
func fieldNames(t reflect.Type) (names map[string]struct{}, ok bool) {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil, false
	}
	if cached, ok := fieldNamesCache.Load(t); ok {
		return cached.(map[string]struct{}), true
	}

	names = map[string]struct{}{}
	collectFieldNames(t, names, map[reflect.Type]bool{})
	fieldNamesCache.Store(t, names)
	return names, true
}

func collectFieldNames(t reflect.Type, names map[string]struct{}, visited map[reflect.Type]bool) {
	if visited[t] {
		return
	}
	visited[t] = true

	for f := range t.Fields() {
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")

		if f.Anonymous && name == "" {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				// The fields of an embedded struct are promoted (even if the struct type is unexported).
				collectFieldNames(ft, names, visited)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		names[name] = struct{}{}
	}
}
