/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package binder

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	/**#bean*/
	"demo/framework/internals/helpers" /*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	/**#bean*/ structure "demo/framework/internals/struct" /*#bean.replace(structure "{{ .PkgPath }}/framework/internals/struct")**/

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// CustomBinder is an implementation of the Binder interface
// which only decodes JSON body and disallow unknow JSON fields.
type CustomBinder struct{}

// Bind implements the `Echo#Binder#Bind` function. It only decodes the JSON body for now.
// Extends it if you also need path or query params. Please reference the Echo#Binder to check how to do it.
func (cb *CustomBinder) Bind(i interface{}, c echo.Context) (err error) {

	req := c.Request()

	if req.ContentLength == 0 {

		return
	}

	ctype := req.Header.Get(echo.HeaderContentType)

	switch {

	case strings.HasPrefix(ctype, echo.MIMEApplicationJSON) && (req.Method == http.MethodPost || req.Method == http.MethodPut):

		bodyBytes := bytes.NewBuffer(make([]byte, 0))
		reader := io.TeeReader(req.Body, bodyBytes)

		jc := json.NewDecoder(reader)
		jc.DisallowUnknownFields()

		if err = jc.Decode(i); err != nil {

			return err
		}

		// Restore the io.ReadCloser to its original state so that we can read c.Request().Body somewhere else.
		c.Request().Body = ioutil.NopCloser(bodyBytes)

		data, _ := helpers.PostDataStripTags(c, false)

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
