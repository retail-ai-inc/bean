/**#bean*/ /*#bean.replace({{ .Copyright }})**/

package error

import (
	/**#bean*/
	bvalidator "demo/framework/internals/validator"
	/*#bean.replace(bvalidator "{{ .PkgPath }}/framework/internals/validator")**/
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorHandlerChain(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandlerChain(
		FakeErrorHandlerMiddleware,
		ValidationErrorHanderMiddleware,
		APIErrorHanderMiddleware,
		HTTPErrorHanderMiddleware,
		DefaultErrorHanderMiddleware)

	//assert fake error
	fakeErr := newFakeError("fake error message")
	checkFakeErr := func(response *httptest.ResponseRecorder, err error) {
		assert.Equal(t, fakeErr, err)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
		var res errorResp
		e := json.Unmarshal(response.Body.Bytes(), &res)
		assert.Nil(t, e)
		assert.Equal(t, ErrorCode("fakeErrorCode"), res.ErrorCode)
		assert.Nil(t, res.Errors)
		assert.Equal(t, fakeErr.Error(), res.ErrorMsg)
	}
	run(e, fakeErr, checkFakeErr)

	//assert api error
	apiErr := NewAPIError(http.StatusUnauthorized, UNAUTHORIZED_ACCESS, errors.New("UNAUTHORIZED"))
	checkAPIErr := func(response *httptest.ResponseRecorder, err error) {
		assert.Equal(t, apiErr, err)
		assert.Equal(t, http.StatusUnauthorized, response.Code)
		var res errorResp
		e := json.Unmarshal(response.Body.Bytes(), &res)
		assert.Nil(t, e)
		assert.Equal(t, UNAUTHORIZED_ACCESS, res.ErrorCode)
		assert.Nil(t, res.Errors)
		assert.Equal(t, apiErr.Error(), res.ErrorMsg)
	}
	run(e, apiErr, checkAPIErr)

	//assert validation error
	validationErr := &bvalidator.ValidationError{
		Err: validator.ValidationErrors([]validator.FieldError{}),
	}
	checkValidationErr := func(response *httptest.ResponseRecorder, err error) {
		assert.Equal(t, validationErr, err)
		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
		var res errorResp
		e := json.Unmarshal(response.Body.Bytes(), &res)
		assert.Nil(t, e)
		assert.Equal(t, API_DATA_VALIDATION_FAILED, res.ErrorCode)
		assert.Nil(t, res.ErrorMsg)
	}
	run(e, validationErr, checkValidationErr)

	//assert http error
	httpErr := &echo.HTTPError{
		Code:    http.StatusNotFound,
		Message: "404 Not Found",
	}
	checkHttpErr := func(response *httptest.ResponseRecorder, err error) {
		assert.Equal(t, httpErr, err)
		assert.Equal(t, http.StatusNotFound, response.Code)
		var res errorResp
		e := json.Unmarshal(response.Body.Bytes(), &res)
		assert.Nil(t, e)
		assert.Equal(t, UNKNOWN_ERROR_CODE, res.ErrorCode)
		assert.Nil(t, res.Errors)
		assert.Equal(t, httpErr.Message, res.ErrorMsg)
	}
	run(e, httpErr, checkHttpErr)

	//assert default error
	defaultErr := errors.New("default error")
	checkDefaultErr := func(response *httptest.ResponseRecorder, err error) {
		assert.Equal(t, defaultErr, err)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
		var res errorResp
		e := json.Unmarshal(response.Body.Bytes(), &res)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.Nil(t, e)
		assert.Equal(t, INTERNAL_SERVER_ERROR, res.ErrorCode)
		assert.Nil(t, res.Errors)
		assert.Nil(t, res.ErrorMsg)
	}
	run(e, defaultErr, checkDefaultErr)
}

func FakeErrorHandlerMiddleware(e error, c echo.Context) (bool, error) {
	he, ok := e.(*fakeError)
	if !ok {
		return false, nil
	}

	err := c.JSON(http.StatusInternalServerError, errorResp{
		ErrorCode: "fakeErrorCode",
		Errors:    nil,
		ErrorMsg:  he.Error(),
	})

	return ok, err
}

func run(e *echo.Echo,
	err error,
	assertFunc func(response *httptest.ResponseRecorder, err error),
) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	runWithRequest(e, req, err, assertFunc)
}

func runWithRequest(e *echo.Echo,
	req *http.Request,
	er error,
	assertFunc func(response *httptest.ResponseRecorder, err error),
) {
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := func(c echo.Context) error {
		return er
	}(c)
	e.HTTPErrorHandler(err, c)
	assertFunc(rec, err)
}

type fakeError struct {
	Message string
}

func (f *fakeError) Error() string {
	return f.Message
}

func newFakeError(msg string) error {
	return &fakeError{
		Message: msg,
	}
}
