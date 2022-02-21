{{ .Copyright }}
package commands

import (
	"fmt"

	"{{ .PkgPath }}/middlewares"
	"{{ .PkgPath }}/routers"
	"{{ .PkgPath }}/validations"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/framework/bean"
	berror "github.com/retail-ai-inc/bean/framework/internals/error"
	"github.com/retail-ai-inc/bean/framework/internals/helpers"
	"github.com/retail-ai-inc/bean/framework/options"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Used for flags.
	host string
	port string

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
	defaultHost := viper.GetString("http.host")
	defaultPort := viper.GetString("http.port")
	startCmd.Flags().StringVar(&host, "host", defaultHost, "host address")
	startCmd.Flags().StringVar(&port, "port", defaultPort, "port number")
}

func start(cmd *cobra.Command, args []string) {
	// Unmarshal the env.json into config object.
	var config bean.Config
	if err := viper.Unmarshal(&config); err != nil {
		fmt.Println(err)
	}

	// Flush buffered sentry events before the program terminates.
	defer sentry.Flush(config.Sentry.Timeout)

	// Prepare sentry options before initialize bean.
	if config.Sentry.On {
		options.SentryOn = true
		config.Sentry.ClientOptions = &sentry.ClientOptions{
			Debug:            config.Sentry.Debug,
			Dsn:              config.Sentry.Dsn,
			Environment:      config.Environment,
			BeforeSend:       options.DefaultBeforeSend, // Custom beforeSend function
			AttachStacktrace: true,
			TracesSampleRate: helpers.FloatInRange(config.Sentry.TracesSampleRate, 0.0, 1.0),
		}

		// Example of setting a global scope, if you want to set the scope per event,
		// please check `sentry.WithScope()`.
		// config.Sentry.ConfigureScope = func(scope *sentry.Scope) {
		// scope.SetTag("my-tag", "my value")
		// }
	}

	// Create a bean object
	b := bean.New(config)

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
		berror.ValidationErrorHanderFunc,
		berror.APIErrorHanderFunc,
		berror.EchoHTTPErrorHanderFunc,
		// Set your custom error handler func here, for example:
		// func(e error, c echo.Context) (bool, error) {
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

	b.ServeAt(host, port)
}
