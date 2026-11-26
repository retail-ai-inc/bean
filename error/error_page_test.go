package error

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	validatorV10 "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/retail-ai-inc/bean/v3/internal/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingRenderer struct{ name string }

func (r *recordingRenderer) Render(_ *echo.Context, _ io.Writer, name string, _ any) error {
	r.name = name
	return nil
}

func customPage(key, value, html string) config.ErrorPage {
	var p config.ErrorPage
	p.Json = append(p.Json, struct {
		Key   string
		Value string
	}{Key: key, Value: value})
	p.Html.File = html
	return p
}

func withConfig(t *testing.T, conf *config.Config) {
	t.Helper()
	orig := config.Bean
	config.Bean = conf
	t.Cleanup(func() { config.Bean = orig })
}

func callHandler(t *testing.T, h ErrorHandlerFunc, err error, jsonReq bool) (bool, *httptest.ResponseRecorder, *recordingRenderer) {
	t.Helper()
	e := echo.New()
	renderer := &recordingRenderer{}
	e.Renderer = renderer
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if jsonReq {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	handled, hErr := h(err, e.NewContext(req, rec))
	require.NoError(t, hErr)
	return handled, rec, renderer
}

func TestHTTPErrorHandlerFunc_ErrorPages(t *testing.T) {
	conf := &config.Config{}
	conf.HTTP.ErrorMessage.E404 = customPage("msg", "custom 404", "custom/404")
	conf.HTTP.ErrorMessage.E405 = customPage("msg", "custom 405", "custom/405")
	conf.HTTP.ErrorMessage.E500 = customPage("msg", "custom 500", "custom/500")
	conf.HTTP.ErrorMessage.E504 = customPage("msg", "custom 504", "custom/504")
	conf.HTTP.ErrorMessage.Default = customPage("msg", "custom default", "custom/default")

	tests := []struct {
		name     string
		err      error
		wantCode int
		wantKey  string
	}{
		{"404 sentinel", echo.ErrNotFound, http.StatusNotFound, "404"},
		{"405 sentinel", echo.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "405"},
		{"500", echo.NewHTTPError(http.StatusInternalServerError, "boom"), http.StatusInternalServerError, "500"},
		{"504", echo.NewHTTPError(http.StatusGatewayTimeout, "timeout").Wrap(errors.New("deadline")), http.StatusGatewayTimeout, "504"},
		{"other status uses default", echo.NewHTTPError(http.StatusUnauthorized, "no"), http.StatusUnauthorized, "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" configured", func(t *testing.T) {
			withConfig(t, conf)

			handled, rec, _ := callHandler(t, HTTPErrorHandlerFunc, tt.err, true)
			require.True(t, handled)
			assert.Equal(t, tt.wantCode, rec.Code)
			assert.JSONEq(t, `{"msg":"custom `+tt.wantKey+`"}`, rec.Body.String())

			handled, rec, renderer := callHandler(t, HTTPErrorHandlerFunc, tt.err, false)
			require.True(t, handled)
			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Equal(t, "custom/"+tt.wantKey, renderer.name)
		})
	}

	defaults := map[string]struct {
		code ErrorCode
		html string
	}{
		"404":     {RESOURCE_NOT_FOUND, "errors/html/404"},
		"405":     {METHOD_NOT_ALLOWED, "errors/html/405"},
		"500":     {INTERNAL_SERVER_ERROR, "errors/html/500"},
		"504":     {TIMEOUT, "errors/html/504"},
		"default": {INTERNAL_SERVER_ERROR, "errors/html/500"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" not configured", func(t *testing.T) {
			withConfig(t, nil)
			want := defaults[tt.wantKey]

			_, rec, _ := callHandler(t, HTTPErrorHandlerFunc, tt.err, true)
			assert.Equal(t, tt.wantCode, rec.Code)
			var resp ErrorResp
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, want.code, resp.ErrorCode)
			assert.NotEmpty(t, resp.ErrorMsg)

			_, _, renderer := callHandler(t, HTTPErrorHandlerFunc, tt.err, false)
			assert.Equal(t, want.html, renderer.name)
		})
	}
}

func TestDefaultErrorHandlerFunc_ErrorPage(t *testing.T) {
	conf := &config.Config{}
	conf.HTTP.ErrorMessage.Default = customPage("msg", "custom default", "custom/default")
	withConfig(t, conf)

	_, rec, _ := callHandler(t, DefaultErrorHandlerFunc, errors.New("any"), true)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"msg":"custom default"}`, rec.Body.String())

	_, _, renderer := callHandler(t, DefaultErrorHandlerFunc, errors.New("any"), false)
	assert.Equal(t, "custom/default", renderer.name)
}

func TestHTTPErrorHandlerFunc_SkipsAPIAndValidationErrors(t *testing.T) {
	withConfig(t, nil)

	for _, err := range []error{
		NewAPIError(http.StatusBadRequest, API_DATA_VALIDATION_FAILED, errors.New("bad")),
		&validator.ValidationError{Err: validatorV10.ValidationErrors{}},
	} {
		handled, _, _ := callHandler(t, HTTPErrorHandlerFunc, err, true)
		assert.False(t, handled, "%T must be handled by its own handler", err)
	}
}

func TestErrorsImplementHTTPStatusCoder(t *testing.T) {
	assert.Equal(t, http.StatusConflict, echo.StatusCode(NewAPIError(http.StatusConflict, INTERNAL_SERVER_ERROR, errors.New("x"))))
	assert.Equal(t, http.StatusBadRequest, echo.StatusCode(&validator.ValidationError{Err: errors.New("x")}))
}
