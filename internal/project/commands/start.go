/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package commands

import (
	/**#bean*/
	"demo/framework/bean"
	/*#bean.replace("{{ .PkgPath }}/framework/bean")**/
	/**#bean*/
	beanValidator "demo/framework/internals/validator"
	/*#bean.replace(beanValidator "{{ .PkgPath }}/framework/internals/validator")**/
	/**#bean*/
	"demo/routers"
	/*#bean.replace("{{ .PkgPath }}/routers")**/

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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	defaultHost := viper.GetString("http.host")
	defaultPort := viper.GetString("http.port")
	startCmd.Flags().StringVar(&host, "host", defaultHost, "host address")
	startCmd.Flags().StringVar(&port, "port", defaultPort, "port number")
}

func start(cmd *cobra.Command, args []string) {
	b := new(bean.Bean)

	b.InitDB()

	b.BeforeBootstrap = func() {
		// init global middleware if you need
		// middlerwares.Init()

		// init router
		routers.Init()
	}

	// Below is an example of how you can initialize your own validator. Just create a new directory
	// as `packages/validator` and create a validator package inside the directory. Then initialize your
	// own validation function here, such as; `validator.MyTestValidationFunction(c, vd)`.
	b.Validate = func(c echo.Context, vd *validator.Validate) {
		beanValidator.TestUserIdValidation(c, vd)

		// Add your own validation function here.
	}

	// Below is an example of how you can set custom error handler middleware
	// bean can call `UseErrorHandlerMiddleware` multiple times
	b.UseErrorHandlerMiddleware(func(e error, c echo.Context) (bool, error) {
		return false, nil
	})

	b.ServeAt(host, port)
}
