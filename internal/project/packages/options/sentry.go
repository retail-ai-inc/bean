package options

import (
	/**#bean*/
	"demo/framework/internals/error"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/error")**/
	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	/**#bean*/
	"demo/framework/internals/validator"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/validator")**/

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

var DefaultSentryClientOptions = sentry.ClientOptions{
	Release:          helpers.CurrVersion(),
	Dsn:              viper.GetString("sentry.dsn"),
	BeforeSend:       beforeSend,       // Custom beforeSend function
	BeforeBreadcrumb: beforeBreadcrumb, // Custom beforeBreadcrumb function
	AttachStacktrace: viper.GetBool("sentry.attachStacktrace"),
	TracesSampleRate: helpers.FloatInRange(viper.GetFloat64("sentry.tracesSampleRate"), 0.0, 1.0),
}

func ConfigureScope(scope *sentry.Scope) {
	// Set your parent scope here, for example:
	// scope.SetTag("my-tag", "my value")
	// scope.SetUser(sentry.User{
	// 	ID: "42",
	// 	Email: "john.doe@example.com",
	// })
	// scope.SetContext("character", map[string]interface{}{
	// 	"name":        "Mighty Fighter",
	// 	"age":         19,
	// 	"attack_type": "melee",
	// })
}

func beforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	// Add any aditional data to the event in here.
	switch err := hint.OriginalException.(type) {
	case *validator.ValidationError:
		event.Contexts["example section"] = map[string]interface{}{
			"example key": "example value",
		}
		return event
	case *error.APIError:
		if err.HTTPStatusCode >= 404 {
			// sentry.PushData(c, he, nil, true)
		}
		return event
	case *echo.HTTPError:
		return event
	default:
		return event
	}
}

func beforeBreadcrumb(breadcrumb *sentry.Breadcrumb, hint *sentry.BreadcrumbHint) *sentry.Breadcrumb {

	return breadcrumb
}
