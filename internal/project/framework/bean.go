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
	/**#bean*/
	berror "demo/framework/internals/error"
	/*#bean.replace(berror "{{ .PkgPath }}/framework/internals/error")**/
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

type Bean struct {
	Validate                func(c echo.Context, vd *validator.Validate)
	BeforeBootstrap         func()
	errorHandlerMiddlewares []berror.ErrorHandlerMiddleware
}

func (b *Bean) Bootstrap() {
	// Create a new echo instance
	e := bootstrap.New()

	errorHandlerMiddlewares := concatErrorHandlerMiddlewares(b.errorHandlerMiddlewares,
		berror.ValidationErrorHanderMiddleware,
		berror.APIErrorHanderMiddleware,
		berror.HTTPErrorHanderMiddleware,
		berror.DefaultErrorHanderMiddleware)
	e.HTTPErrorHandler = berror.ErrorHandlerChain(errorHandlerMiddlewares...)
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

func (b *Bean) UseErrorHandlerMiddleware(errorHandlerMiddleware berror.ErrorHandlerMiddleware) {
	if b.errorHandlerMiddlewares == nil {
		b.errorHandlerMiddlewares = []berror.ErrorHandlerMiddleware{}
	}
	b.errorHandlerMiddlewares = append(b.errorHandlerMiddlewares, errorHandlerMiddleware)
}

func concatErrorHandlerMiddlewares(middlewares []berror.ErrorHandlerMiddleware, frameworkMiddlewares ...berror.ErrorHandlerMiddleware) []berror.ErrorHandlerMiddleware {
	if middlewares == nil {
		middlewares = []berror.ErrorHandlerMiddleware{}
	}
	for _, m := range frameworkMiddlewares {
		middlewares = append(middlewares, m)
	}
	return middlewares
}
