package bean

import (
	"github.com/labstack/echo/v4"
)

type echoContext struct {
	bContext
}

// func (c *echoContext) Request() *http.Request {
// 	// using bContext.Request
// 	// TODO implement me
// 	panic("implement me")
// }

// WrapEchoHandler wraps `HandlerFunc` into `echo.HandlerFunc`.
func WrapEchoHandler(h HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		request := c.Request()
		response := c.Response()
		ctx := pool.Get().(*echoContext)
		ctx.Reset(request, response)
		err = h(ctx)
		return err
	}
}

// WrapEchoMiddleware wraps `func(bean.HandlerFunc) bean.HandlerFunc` into `echo.MiddlewareFunc`
func WrapEchoMiddleware(m MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			request := c.Request()
			response := c.Response()
			bc := pool.Get().(*echoContext)
			bc.Reset(request, response)
			return m(func(ctx Context) error {
				c.SetRequest(ctx.Request())
				c.SetResponse(echo.NewResponse(ctx.Response(), c.Echo()))
				return next(c)
			})(bc)
		}
	}
}
