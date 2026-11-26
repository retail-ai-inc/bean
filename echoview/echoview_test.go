package echoview

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/retail-ai-inc/bean/v3/goview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With the middleware, `Render` must use the given status code and the HTML content type.
func TestRender_MiddlewareUsesStatusCode(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "page.html"), []byte(`page {{.user}}`), 0o644))

	cfg := goview.DefaultConfig
	cfg.Root = root
	e := echo.New()
	e.Use(NewMiddleware(cfg))
	e.GET("/", func(c *echo.Context) error {
		return Render(c, http.StatusNotFound, "page", map[string]any{"user": "x"})
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "page x", rec.Body.String())
	assert.Contains(t, rec.Header().Get(echo.HeaderContentType), "text/html")
}
