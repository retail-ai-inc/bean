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

// SentryCaptureExceptionWithEcho captures an exception with echo context and send to sentry if sentry is configured.
// It caputures the exception even if the context or sentry hub in the context is nil.
// To capture an exception with a stack trace, include the top-level error.
// For supported libraries, see: https://pkg.go.dev/github.com/getsentry/sentry-go@v0.30.0#ExtractStacktrace
func SentryCaptureExceptionWithEcho(c echo.Context, err error) {

	sentryCaptureException(err, false, func() (*sentry.Hub, bool) {

		if c != nil {
			return sentryecho.GetHubFromContext(c), true
		}

		return nil, false
	})
}

// SentryCaptureException captures an exception with context and send to sentry if sentry is configured.
// It caputures the exception even if the context or sentry hub in the context is nil.
// To capture an exception with a stack trace, include the top-level error.
// For supported libraries, see: https://pkg.go.dev/github.com/getsentry/sentry-go@v0.30.0#ExtractStacktrace
func SentryCaptureException(ctx context.Context, err error) {

	sentryCaptureException(err, false, func() (*sentry.Hub, bool) {

		if ctx != nil {
			return sentry.GetHubFromContext(ctx), true
		}

		return nil, false
	})
}

// LogAndSentryCaptureException logs the error and captures an exception with context and send to sentry if sentry is configured.
// It caputures the exception even if the context or sentry hub in the context is nil.
func LogAndSentryCaptureException(ctx context.Context, err error) {

	sentryCaptureException(err, true, func() (*sentry.Hub, bool) {

		if ctx != nil {
			return sentry.GetHubFromContext(ctx), true
		}

		return nil, false
	})
}

func sentryCaptureException(err error, logging bool, getHub func() (hub *sentry.Hub, addMissHubInfo bool)) {

	if err == nil {
		return
	}

	// Log the error if logging is on, whether sentry is on or off.
	if logging {
		log.Logger().Error(err)
	}

	if !config.Bean.Sentry.On {
		return
	}

	hub, addMissHubInfo := getHub()
	if hub == nil {
		wrapErr := err
		if addMissHubInfo {
			wrapErr = fmt.Errorf("context is missing hub information: %w", err)
		}
		// Capture the exception without context even if the hub is nil.
		sentry.CurrentHub().Clone().CaptureException(wrapErr)
		return
	}

	hub.CaptureException(err)
}

// SentryCaptureMessageWithEcho captures a message with echo context and send to sentry.
// It captures the message even if the context or sentry hub in the context is nil.
func SentryCaptureMessageWithEcho(c echo.Context, msg string) {

	sentryCaptureMsg(msg, false, func() (*sentry.Hub, bool) {

		if c != nil {
			return sentryecho.GetHubFromContext(c), true
		}

		return nil, false
	})
}

// SentryCaptureMessage captures a message with context and send to sentry if sentry is configured.
// It captures the message even if the context or sentry hub in the context is nil.
func SentryCaptureMessage(ctx context.Context, msg string) {

	sentryCaptureMsg(msg, false, func() (*sentry.Hub, bool) {

		if ctx != nil {
			return sentry.GetHubFromContext(ctx), true
		}

		return nil, false
	})
}

// LogAndSentryCaptureMessage logs the message and captures a message with context and send to sentry if sentry is configured.
// It captures the message without context even if the context or sentry hub in the context is nil.
func LogAndSentryCaptureMessage(ctx context.Context, msg string) {

	sentryCaptureMsg(msg, true, func() (*sentry.Hub, bool) {

		if ctx != nil {
			return sentry.GetHubFromContext(ctx), true
		}

		return nil, false
	})
}

func sentryCaptureMsg(msg string, logging bool, getHub func() (hub *sentry.Hub, addMissHubInfo bool)) {

	if msg == "" {
		return
	}

	// Log the message if logging is on, whether sentry is on or off.
	if logging {
		log.Logger().Info(msg)
	}

	if !config.Bean.Sentry.On {
		return
	}

	hub, addMissHubInfo := getHub()
	if hub == nil {
		wrapMsg := msg
		if addMissHubInfo {
			wrapMsg = fmt.Sprintf("context is missing hub information: %s", msg)
		}
		// Capture the message without context even if the hub is nil.
		sentry.CurrentHub().Clone().CaptureMessage(wrapMsg)
		return
	}

	hub.CaptureMessage(msg)
}
