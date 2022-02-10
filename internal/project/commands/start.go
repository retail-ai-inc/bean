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
	beanValidator "demo/framework/internals/validator"
	/*#bean.replace(beanValidator "{{ .PkgPath }}/framework/internals/validator")**/
	/**#bean*/
	"demo/routers"
	/*#bean.replace("{{ .PkgPath }}/routers")**/

	"github.com/getsentry/sentry-go"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
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
	// Set the timeout to the maximum duration the program can afford to wait.
	defer sentry.Flush(viper.GetDuration("sentry.timeout"))

	// Create a bean object
	b := bean.New()

	// Below is an example of how you can initialize your own validator. Just create a new directory
	// as `packages/validator` and create a validator package inside the directory. Then initialize your
	// own validation function here, such as; `validator.MyTestValidationFunction(c, vd)`.
	b.Validate = func(c echo.Context, vd *validator.Validate) {
		beanValidator.TestUserIdValidation(c, vd)
		// Add your own validation function here.
	}

	// Set custom middleware in here.
	b.UseMiddlewares(
	// Example:
	// func(arg string) echo.MiddlewareFunc {
	// 	return func(next echo.HandlerFunc) echo.HandlerFunc {
	// 		return func(c echo.Context) error {
	// 			c.Logger().Info(arg)
	// 			return next(c)
	// 		}
	// 	}
	// }("example"),
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

		// You can also replace the default error handler.
		// b.Echo.HTTPErrorHandler = YourErrorHandler()
	}

	b.ServeAt(host, port)
}
