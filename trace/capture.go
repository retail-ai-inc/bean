package trace

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/config"
	"github.com/retail-ai-inc/bean/v2/log"
)

// SentryCaptureExceptionWithEcho captures an exception with echo context and send to sentry.
// This is a global function to send sentry exception if you configure the sentry through env.json. You cann pass a proper context or nil.
func SentryCaptureExceptionWithEcho(c echo.Context, err error) {

	if err == nil {
		return
	}

	if !config.Bean.Sentry.On {
		log.Logger().Error(err)
		return
	}

	if c != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CurrentHub().Clone().CaptureException(fmt.Errorf("echo context is missing hub information: %w", err))
		}

		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureException(err)
}

// SentryCaptureMessageWithEcho captures a message with echo context and send to sentry.
// This is a global function to send sentry message if you configure the sentry through env.json. You cann pass a proper context or nil.
func SentryCaptureMessageWithEcho(c echo.Context, msg string) {

	if msg == "" {
		return
	}

	if !config.Bean.Sentry.On {
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

// SentryCaptureException captures an exception with context and send to sentry if sentry is configured.
func SentryCaptureException(ctx context.Context, err error) {

	if err == nil {
		return
	}

	if !config.Bean.Sentry.On {
		return
	}

	if ctx != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CurrentHub().Clone().CaptureException(fmt.Errorf("context is missing hub information: %w", err))
		}
		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureException(err)
}

// SentryCaptureMessage captures a message with context and send to sentry if sentry is configured.
func SentryCaptureMessage(ctx context.Context, msg string) {

	if msg == "" {
		return
	}

	if !config.Bean.Sentry.On {
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

// LogAndSentryCaptureException logs the error and captures an exception with context and send to sentry if sentry is configured.
func LogAndSentryCaptureException(ctx context.Context, err error) {

	if err == nil {
		return
	}

	// Log the error first whether sentry is on or off.
	log.Logger().Error(err)

	if !config.Bean.Sentry.On {
		return
	}

	if ctx != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CurrentHub().Clone().CaptureException(fmt.Errorf("context is missing hub information: %w", err))
		}
		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureException(err)
}

// LogAndSentryCaptureMessage logs the message and captures a message with context and send to sentry if sentry is configured.
func LogAndSentryCaptureMessage(ctx context.Context, msg string) {

	if msg == "" {
		return
	}

	// Log the message first whether sentry is on or off.
	log.Logger().Info(msg)

	if !config.Bean.Sentry.On {
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
