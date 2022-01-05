/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package framework

import (
	"net/http"

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
	Validate        func(c echo.Context, vd *validator.Validate)
	BeforeBootstrap func()
}

func (b *Bean) Bootstrap() {
	// Create a new echo instance
	e := bootstrap.New()

	// before bean bootstrap
	if b.BeforeBootstrap != nil {
		b.BeforeBootstrap()
	}

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
