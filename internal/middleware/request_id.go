package middleware

import (
	"github.com/labstack/echo/v4"
	bctx "github.com/retail-ai-inc/bean/v2/context"
)

var RequestIDHandler = func(c echo.Context, requestID string) {
	// Set the request ID in the request header if not set.
	if v := c.Request().Header.Get(echo.HeaderXRequestID); v == "" {
		c.Request().Header.Set(echo.HeaderXRequestID, requestID)
	}

	// Set the request ID in the echo context if not set.
	if v, ok := c.Get(echo.HeaderXRequestID).(string); !ok || v == "" {
		c.Set(echo.HeaderXRequestID, requestID)
	}

	ctx := c.Request().Context()

	if _, ok := bctx.GetRequestID(ctx); !ok {
		// Set the request ID in the context as well if not found.
		ctx = bctx.SetRequestID(ctx, requestID)
	}

	c.SetRequest(c.Request().WithContext(ctx))
}
