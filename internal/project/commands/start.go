/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package commands

import (
	/**#bean*/
	"demo/framework/bean"
	/*#bean.replace("{{ .PkgPath }}/framework/bean")**/
	/**#bean*/
	berror "demo/framework/internals/error"
	/*#bean.replace(berror "{{ .PkgPath }}/framework/internals/error")**/
	/**#bean*/
	"demo/middlewares"
	/*#bean.replace("{{ .PkgPath }}/middlewares")**/
	/**#bean*/
	"demo/routers"
	/*#bean.replace("{{ .PkgPath }}/routers")**/
	/**#bean*/
	"demo/validations"
	/*#bean.replace("{{ .PkgPath }}/validations")**/

	"github.com/getsentry/sentry-go"
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
	// Flush buffered sentry events before the program terminates.
	defer sentry.Flush(viper.GetDuration("sentry.timeout"))

	// Create a bean object
	b := bean.New()

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
		// b.Echo.Validator = &CustomerValidaer{}
	}

	b.ServeAt(host, port)
}
