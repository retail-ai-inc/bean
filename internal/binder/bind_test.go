package binder

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	berror "github.com/retail-ai-inc/bean/v3/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Base struct {
	ID int `json:"id"`
}

type payload struct {
	Base
	Name    string `json:"name"`
	NoTag   string
	Skipped string `json:"-"`
	private string //nolint:unused
}

func bind(t *testing.T, body string, dst any) error {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := echo.New().NewContext(req, httptest.NewRecorder())

	err := (&CustomBinder{}).Bind(c, dst)

	// The body can be read again.
	again, readErr := io.ReadAll(c.Request().Body)
	require.NoError(t, readErr)
	assert.Equal(t, body, string(again))
	return err
}

func TestBind_Valid(t *testing.T) {
	var p payload
	require.NoError(t, bind(t, `{"id":1,"name":"a","NoTag":"b"}`, &p))
	assert.Equal(t, payload{Base: Base{ID: 1}, Name: "a", NoTag: "b"}, p)

	// A map target accepts any field.
	var m map[string]any
	require.NoError(t, bind(t, `{"anything":1}`, &m))
	assert.Equal(t, map[string]any{"anything": float64(1)}, m)
}

func TestBind_InvalidBodyIs400(t *testing.T) {
	for name, body := range map[string]string{
		"syntax error":           `{"name":`,
		"unknown field":          `{"name":"a","other":1}`,
		"field name case":        `{"Name":"a"}`,
		"skipped field":          `{"Skipped":"a"}`,
		"trailing data":          `{"name":"a"} garbage`,
		"second value":           `{"name":"a"}{"name":"b"}`,
		"wrong type":             `{"id":"1"}`,
		"embedded field by case": `{"ID":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			var p payload
			err := bind(t, body, &p)
			var ae *berror.APIError
			require.True(t, errors.As(err, &ae), "got %v", err)
			assert.Equal(t, http.StatusBadRequest, ae.HTTPStatusCode)
			assert.Equal(t, berror.PROBLEM_PARSING_JSON, ae.GlobalErrCode)
		})
	}
}
