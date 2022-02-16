// Copyright The RAI Inc.
// The RAI Authors
package options

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/framework/internals/error"
	"github.com/retail-ai-inc/bean/framework/internals/validator"
)

type SentryConfig struct {
	On               bool
	Debug            bool
	Dsn              string
	Timeout          time.Duration
	TracesSampleRate float64
	ClientOptions    *sentry.ClientOptions
	ConfigureScope   func(scope *sentry.Scope)
}

var SentryOn bool // Global variable

// Modify event through beforeSend function.
func DefaultBeforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	// Example: enriching the event by adding aditional data.
	switch err := hint.OriginalException.(type) {
	case *validator.ValidationError:
		return event
	case *error.APIError:
		event.Contexts["Error"] = map[string]interface{}{
			"HTTPStatusCode": err.HTTPStatusCode,
			"GlobalErrCode":  err.GlobalErrCode,
			"Message":        err.Error(),
		}
		return event
	case *echo.HTTPError:
		return event
	default:
		return event
	}
}

// Modify breadcrumbs through beforeBreadcrumb function.
func DefaultBeforeBreadcrumb(breadcrumb *sentry.Breadcrumb, hint *sentry.BreadcrumbHint) *sentry.Breadcrumb {
	// Example: discard the breadcrumb by return nil.
	// if breadcrumb.Category == "example" {
	// 	return nil
	// }
	return breadcrumb
}
