package middleware

import (
	bctx "github.com/retail-ai-inc/bean/v2/context"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
)

// SetHubToContext sets the sentry hub to the context.
var SetHubToContext = func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		ctx := c.Request().Context()

		if sentry.GetHubFromContext(ctx) == nil {
			if hub := sentryecho.GetHubFromContext(c); hub != nil {
				// Set sentry hub to the context as well as the echo context if not found.
				// so that you can take it out from the context, too.
				ctx = sentry.SetHubOnContext(ctx, hub)
			}
		}

		if _, ok := bctx.GetRequest(ctx); !ok {
			// Set request to the context as well as the echo context if not found.
			// The request may be used later for tracing an aync task spawned by the request.
			ctx = bctx.SetRequest(ctx, c.Request())
		}

		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}
