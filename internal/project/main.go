/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package main

import (
	/**#bean*/
	"demo/framework"
	/*#bean.replace("{{ .PkgPath }}/framework")**/
	/**#bean*/
	beanValidator "demo/framework/internals/validator"
	/*#bean.replace(beanValidator "{{ .PkgPath }}/framework/internals/validator")**/
	/**#bean*/
	"demo/routers"
	/*#bean.replace("{{ .PkgPath }}/routers")**/

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func main() {

	bean := new(framework.Bean)

	bean.BeforeBootstrap = func() {
		// init global middleware if you need
		// middlerwares.Init()

		// init router
		routers.Init()
	}

	// Below is an example of how you can initialize your own validator. Just create a new directory
	// as `packages/validator` and create a validator package inside the directory. Then initialize your
	// own validation function here, such as; `validator.MyTestValidationFunction(c, vd)`.
	bean.Validate = func(c echo.Context, vd *validator.Validate) {
		beanValidator.TestUserIdValidation(c, vd)

		// Add your own validation function here.
	}

	// Below is an example of how you can set custom error handler middleware
	// bean can call `UseErrorHandlerMiddleware` multiple times
	bean.UseErrorHandlerMiddleware(func(e error, c echo.Context) (bool, error) {
		return false, nil
	})

	bean.Bootstrap()
}
