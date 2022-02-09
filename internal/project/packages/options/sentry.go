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

func DefaultSentryClientOptions() sentry.ClientOptions {
	return sentry.ClientOptions{
		Debug:            viper.GetBool("sentry.debug"),
		Dsn:              viper.GetString("sentry.dsn"),
		Environment:      viper.GetString("environment"),
		BeforeSend:       beforeSend,       // Custom beforeSend function
		BeforeBreadcrumb: beforeBreadcrumb, // Custom beforeBreadcrumb function
		AttachStacktrace: true,
		TracesSampleRate: helpers.FloatInRange(viper.GetFloat64("sentry.tracesSampleRate"), 0.0, 1.0),
	}
}

// This will set the scope globally, if you want to set the scope per event,
// please check `sentry.WithScope()`.
func ConfigureScope(scope *sentry.Scope) {
	// Set your parent scope here, for example:
	// scope.SetTag("my-tag", "my value")
	// scope.SetUser(sentry.User{
	// 	ID:    "42",
	// 	Email: "john.doe@example.com",
	// })
	// scope.SetContext("character", map[string]interface{}{
	// 	"name":        "Mighty Fighter",
	// 	"age":         19,
	// 	"attack_type": "melee",
	// })
	// scope.AddBreadcrumb(&sentry.Breadcrumb{
	// 	Type:     "debug",
	// 	Category: "scope",
	// 	Message:  "testing scope.AddBreadcrumb()",
	// 	Level:    sentry.LevelInfo,
	// }, 10)
}

func beforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	// You can change or add aditional data to the event in here.
	// Example:
	switch err := hint.OriginalException.(type) {
	case *validator.ValidationError:
		return event
	case *error.APIError:
		return event
	case *echo.HTTPError:
		return event
	default:
		event.Contexts["Error"] = map[string]interface{}{
			"message": err.Error(),
		}
		return event
	}
}

func beforeBreadcrumb(breadcrumb *sentry.Breadcrumb, hint *sentry.BreadcrumbHint) *sentry.Breadcrumb {
	// You can customize breadcrumbs through this beforeBreadcrumb function.
	// Example: discard the breadcrumb by return nil.
	// if breadcrumb.Category == "example" {
	// 	return nil
	// }
	return breadcrumb
}
