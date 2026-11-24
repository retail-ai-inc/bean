package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func namedHandler(c *echo.Context) error { return c.NoContent(http.StatusOK) }

func newEcho(allowed ...string) *echo.Echo {
	return echo.NewWithConfig(echo.Config{
		NoGroupAutoRegister404Routes: true,
		Router:                       New(echo.NewRouter(echo.RouterConfig{AllowOverwritingRoute: true}), allowed, nil),
	})
}

func serve(e *echo.Echo, method, path string) int {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec.Code
}

func routeNames(e *echo.Echo) map[string]string {
	names := map[string]string{}
	for _, r := range e.Router().Routes() {
		names[r.Method+" "+r.Path] = r.Name
	}
	return names
}

func TestRouter_HandlerName(t *testing.T) {
	e := newEcho()
	e.GET("/named", namedHandler)
	_, err := e.AddRoute(echo.Route{Method: http.MethodGet, Path: "/custom", Handler: namedHandler, Name: "custom"})
	require.NoError(t, err)

	names := routeNames(e)
	assert.Equal(t, "github.com/retail-ai-inc/bean/v3/internal/router.namedHandler", names["GET /named"])
	assert.Equal(t, "custom", names["GET /custom"])
	assert.Equal(t, http.StatusOK, serve(e, http.MethodGet, "/named"))
}

func TestRouter_AllowedMethod(t *testing.T) {
	e := newEcho(" get", "POST", "GET")
	e.GET("/users", namedHandler)
	e.DELETE("/users", namedHandler) // not allowed: skipped
	e.PUT("/only-put", namedHandler) // not allowed: skipped

	names := routeNames(e)
	assert.Contains(t, names, "GET /users")
	assert.NotContains(t, names, "DELETE /users")
	assert.NotContains(t, names, "PUT /only-put")

	assert.Equal(t, http.StatusOK, serve(e, http.MethodGet, "/users"))
	assert.Equal(t, http.StatusMethodNotAllowed, serve(e, http.MethodDelete, "/users"))
	assert.Equal(t, http.StatusMethodNotAllowed, serve(e, http.MethodPost, "/users"))
	assert.Equal(t, http.StatusNotFound, serve(e, http.MethodPut, "/only-put"))
	assert.Equal(t, http.StatusNotFound, serve(e, http.MethodGet, "/unknown"))
}

func TestRouter_AnyIsExpandedToAllowedMethods(t *testing.T) {
	e := newEcho("GET", "POST")
	e.Any("/any", namedHandler)

	names := routeNames(e)
	assert.Contains(t, names, "GET /any")
	assert.Contains(t, names, "POST /any")
	assert.NotContains(t, names, echo.RouteAny+" /any")

	assert.Equal(t, http.StatusOK, serve(e, http.MethodGet, "/any"))
	assert.Equal(t, http.StatusOK, serve(e, http.MethodPost, "/any"))
	assert.Equal(t, http.StatusMethodNotAllowed, serve(e, http.MethodDelete, "/any"))

	require.NoError(t, e.Router().Remove(echo.RouteAny, "/any"))
	assert.Empty(t, e.Router().Routes())
}

func TestRouter_EmptyAllowedMethodAllowsAll(t *testing.T) {
	e := newEcho()
	e.DELETE("/users", namedHandler)
	e.Any("/any", namedHandler)

	assert.Equal(t, http.StatusOK, serve(e, http.MethodDelete, "/users"))
	assert.Equal(t, http.StatusOK, serve(e, http.MethodPatch, "/any"))
}

func TestRouter_GroupWrongMethodIs405(t *testing.T) {
	e := newEcho("GET", "POST")
	g := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc { return next })
	g.GET("/items", namedHandler)

	assert.Equal(t, http.StatusOK, serve(e, http.MethodGet, "/api/items"))
	assert.Equal(t, http.StatusMethodNotAllowed, serve(e, http.MethodPost, "/api/items"))
	assert.Equal(t, http.StatusNotFound, serve(e, http.MethodGet, "/api/unknown"))
}

func TestRouter_RouteNotFoundIsKept(t *testing.T) {
	e := newEcho("GET")
	e.RouteNotFound("/static/*", func(c *echo.Context) error { return c.String(http.StatusTeapot, "custom") })

	assert.Equal(t, http.StatusTeapot, serve(e, http.MethodGet, "/static/x"))
}
