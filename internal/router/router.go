// Package router wraps echo's router to keep bean's routing conventions.
package router

import (
	"log/slog"
	"strings"

	"github.com/labstack/echo/v5"
)

// Router wraps an `echo.Router` and applies bean's routing conventions when routes are added:
//
//   - Unnamed routes are named after their handler function (the default in echo v4; echo v5 uses
//     "METHOD:/path"). `route list` and `gen spec` rely on the handler name.
//   - Only the HTTP methods listed in `http.allowedMethod` are registered. Routes with other methods are
//     skipped (with a warning), so requests to them get 404 / 405 from the echo router.
//   - `Any` routes are registered for every allowed method instead of echo's catch-all method, so that a
//     method outside `http.allowedMethod` gets 405 and the routes show up in `route list`.
//
// Because unmatched paths and methods are resolved by the echo router itself (returning `echo.ErrNotFound` /
// `echo.ErrMethodNotAllowed`), bean does not need an extra middleware to detect them.
type Router struct {
	echo.Router
	allowedMethods []string
	allowedSet     map[string]struct{}
	logger         *slog.Logger
}

// New wraps r. If allowedMethods is empty every method is allowed. logger may be nil.
func New(r echo.Router, allowedMethods []string, logger *slog.Logger) *Router {
	router := &Router{
		Router:     r,
		allowedSet: make(map[string]struct{}, len(allowedMethods)),
		logger:     logger,
	}
	for _, m := range allowedMethods {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if _, ok := router.allowedSet[m]; ok {
			continue
		}
		router.allowedSet[m] = struct{}{}
		router.allowedMethods = append(router.allowedMethods, m)
	}
	return router
}

// Add registers the route following bean's routing conventions.
func (r *Router) Add(route echo.Route) (echo.RouteInfo, error) {
	if route.Name == "" && route.Handler != nil {
		route.Name = echo.HandlerName(route.Handler)
	}

	// echo internal "route not found" routes (e.g. registered by `Group.Use`) must be kept as they are.
	if len(r.allowedSet) == 0 || route.Method == echo.RouteNotFound {
		return r.Router.Add(route)
	}

	if route.Method == echo.RouteAny {
		var first echo.RouteInfo
		for i, method := range r.allowedMethods {
			rt := route
			rt.Method = method
			ri, err := r.Router.Add(rt)
			if err != nil {
				return echo.RouteInfo{}, err
			}
			if i == 0 {
				first = ri
			}
		}
		return first, nil
	}

	if !r.isAllowed(route.Method) {
		if r.logger != nil {
			r.logger.Warn("route is not registered because its method is not listed in http.allowedMethod",
				"method", route.Method, "path", route.Path, "name", route.Name)
		}
		return route.ToRouteInfo(nil), nil
	}

	return r.Router.Add(route)
}

// Remove removes the route. `Any` routes are removed for every allowed method.
func (r *Router) Remove(method string, path string) error {
	if method != echo.RouteAny || len(r.allowedSet) == 0 {
		return r.Router.Remove(method, path)
	}

	var firstErr error
	for _, m := range r.allowedMethods {
		if err := r.Router.Remove(m, path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Router) isAllowed(method string) bool {
	_, ok := r.allowedSet[method]
	return ok
}
