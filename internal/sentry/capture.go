package sentry

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
)

func CaptureException(ctx context.Context, err error,
	logger echo.Logger, sentryOn bool, noHubMsg string,
) {
	if err == nil {
		return
	}

	if !sentryOn {
		logger.Error(err)
		return
	}

	if ctx != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CurrentHub().Clone().CaptureException(fmt.Errorf("%s: %w", noHubMsg, err))
		}
		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureException(err)
}

func CaptureExceptionWithEchoCtx(c echo.Context, err error,
	logger echo.Logger, sentryOn bool, noHubMsg string,
) {
	if err == nil {
		return
	}

	if !sentryOn {
		logger.Error(err)
		return
	}

	if c != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CurrentHub().Clone().CaptureException(fmt.Errorf("%s: %w", noHubMsg, err))
		}

		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureException(err)
}

func CaptureMessage(ctx context.Context, msg string, sentryOn bool) {

	if !sentryOn {
		return
	}

	if ctx != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureMessage(msg)
		}

		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureMessage(msg)
}

func CaptureMessageWithEchoCtx(c echo.Context, msg string, sentryOn bool) {

	if !sentryOn {
		return
	}

	if c != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureMessage(msg)
		}

		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureMessage(msg)
}
