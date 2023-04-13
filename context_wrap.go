package bean

import (
	"context"

	"github.com/labstack/echo/v4"
)

type contextKey string

const beanContextKey = contextKey("beanContextKey")

// WrapEchoHandler wraps `HandlerFunc` into `echo.HandlerFunc`.
func WrapEchoHandler(h HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		bc := genBeanContextFromEcho(c)
		err = h(bc)
		return err
	}
}

// WrapEchoMiddleware wraps `func(bean.HandlerFunc) bean.HandlerFunc` into `echo.MiddlewareFunc`
func WrapEchoMiddleware(m MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			bc := genBeanContextFromEcho(c)
			return m(func(ctx Context) error {
				c.SetRequest(ctx.Request())
				c.SetResponse(echo.NewResponse(ctx.Response(), c.Echo()))
				for key, val := range ctx.Keys() {
					c.Set(key, val)
				}
				return next(c)
			})(bc)
		}
	}
}

type binderWrapper struct {
	handler func(i interface{}, c echo.Context) error
	c       echo.Context
}

func (bb *binderWrapper) Bind(i interface{}) error {
	return bb.handler(i, bb.c)
}

func genBeanContextFromEcho(c echo.Context) *beanContext {
	request := c.Request()
	if bc, ok := request.Context().Value(beanContextKey).(*beanContext); ok {
		return bc
	}

	bc := &beanContext{}
	response := c.Response()
	request = request.WithContext(context.WithValue(request.Context(), beanContextKey, bc))
	bc.Reset(request, response)
	bc.SetBinder(&binderWrapper{c.Echo().Binder.Bind, c})
	return bc
}
