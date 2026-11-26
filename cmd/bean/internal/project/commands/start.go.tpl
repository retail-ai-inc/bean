{{ .Copyright }}
package commands

import (
	"log"

	"{{ .PkgPath }}/middlewares"
	"{{ .PkgPath }}/routers"
	"{{ .PkgPath }}/validations"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v3"
	"github.com/retail-ai-inc/bean/v3/config"
	berror "github.com/retail-ai-inc/bean/v3/error"
	"github.com/retail-ai-inc/bean/v3/helpers"
	"github.com/retail-ai-inc/bean/v3/trace"
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	host string
	port string
	startWeb bool
	startQueue bool

	// startCmd represents the start command.
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the web service",
		Long:  `Start the web service base on the configs in env.json`,
		Run:   start,
	}
)

func init() {
	rootCmd.AddCommand(startCmd)
	defaultHost := config.Bean.HTTP.Host
	defaultPort := config.Bean.HTTP.Port
	startCmd.Flags().StringVar(&host, "host", defaultHost, "host address")
	startCmd.Flags().StringVar(&port, "port", defaultPort, "port number")
	startCmd.Flags().BoolVarP(&startWeb, "web", "w", false, "start with web service")
	startCmd.Flags().BoolVarP(&startQueue, "queue", "q", false, "start with job queue worker pool service")
}

func start(cmd *cobra.Command, args []string) {
	// Prepare sentry options before initialize bean.
	if config.Bean.Sentry.On {		
		config.Bean.Sentry.ClientOptions = &sentry.ClientOptions{
			Debug:            config.Bean.Sentry.Debug,
			Dsn:              config.Bean.Sentry.Dsn,
			Environment:      config.Bean.Environment,
			BeforeSend:       trace.DefaultBeforeSend, // Default beforeSend function. You can initialize your own custom function.
			AttachStacktrace: true,
			TracesSampleRate: helpers.FloatInRange(config.Bean.Sentry.TracesSampleRate, 0.0, 1.0),
		}

		// Example of setting a global scope, if you want to set the scope per event,
		// please check `sentry.WithScope()`.
		// config.Bean.Sentry.ConfigureScope = func(scope *sentry.Scope) {
		// scope.SetTag("my-tag", "my value")
		// }
	}

	// Create a bean object
	b, err := bean.New()
	if err != nil {
		log.Fatalf("Failed to initialize bean: %v", err)
	}

	// Add custom validation to the default validator.
	b.UseValidation(
		// Example:
		validations.Example,
	)

	// Set custom middleware in here.
	b.UseMiddlewares(
		// Example:
		middlewares.Example("example middleware"),
	)

	// Set custom error handler function here.
	// Bean use a error function chain inside the default http error handler,
	// so that it can easily add or remove the different kind of error handling.
	b.UseErrorHandlerFuncs(
		berror.ValidationErrorHandlerFunc, // You can use your own custom validation error handler func.
		berror.APIErrorHandlerFunc, // You can use your own custom API error handler func.
		berror.HTTPErrorHandlerFunc,
		// Set your custom error handler func here, for example:
		// func(e error, c *echo.Context) (bool, error) {
		// 	return false, nil
		// },
	)

	b.BeforeServe = func() {
		// Init DB dependency.
		b.InitDB()

		// Init different routes.
		routers.Init(b)

		// You can also replace the default error handler:
		// b.Echo.HTTPErrorHandler = YourErrorHandler()

		// Or default validator:
		// b.Echo.Validator = &CustomValidator{}
	}

	if startQueue && startWeb {
		// Start both web and worker pool
		go func() {
			// TODO: start the queue
		}()
		if err := b.ServeAt(host, port); err != nil {
			log.Fatalf("Failed to start the service: %v", err)
		}
	} else if startQueue && !startWeb {
		// Only start worker pool
		// TODO: start the queue
	} else {
		// Only start the web
		if err := b.ServeAt(host, port); err != nil {
			log.Fatalf("Failed to start the service: %v", err)
		}
	}
}
