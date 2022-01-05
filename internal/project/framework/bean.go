/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package framework

import (
	"fmt"
	"net/http"
	"os"

	/**#bean*/
	"demo/framework/bootstrap"
	/*#bean.replace("{{ .PkgPath }}/framework/bootstrap")**/
	/**#bean*/
	validate "demo/framework/internals/validator"
	/*#bean.replace(validate "{{ .PkgPath }}/framework/internals/validator")**/

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

type Bean struct {
	Router                func(e *echo.Echo)
	Validate              func(c echo.Context, vd *validator.Validate)
	MiddlewareInitializer func(e *echo.Echo)
}

func (b *Bean) Bootstrap() {
	// Create a new echo instance
	e := bootstrap.New()

	// Initialize the middlewares
	if b.MiddlewareInitializer != nil {
		b.MiddlewareInitializer(e)
	}

	// Initialize the router
	if b.Router == nil {
		fmt.Printf("Please set Bean's router")
		os.Exit(1)
	}
	b.Router(e)

	// Initialize and bind the validator to echo instance
	validate.BindCustomValidator(e, b.Validate)

	projectName := viper.GetString("name")

	e.Logger.Info(`Starting ` + projectName + ` server...🚀`)

	listenAt := viper.GetString("http.host") + ":" + viper.GetString("http.port")

	// Start the server
	if err := e.Start(listenAt); err != nil && err != http.ErrServerClosed {
		e.Logger.Fatal(err)
	}
}
