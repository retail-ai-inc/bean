{{ .Copyright }}
package main

import (
	"{{ .PkgPath }}/framework"
	beanValidator "{{ .PkgPath }}/framework/internals/validator"
	"{{ .PkgPath }}/routers"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func main() {

	bean := new(framework.Bean)
	bean.Router = routers.Init
	
	// You can initialize your own middleware below.
	bean.MiddlewareInitializer = nil
	
	// Below is an example of how you can initialize your own validator. Just create a new directory
	// as `packages/validator` and create a validator package inside the directory. Then initialize your
	// own validation function here, such as; `validator.MyTestValidationFunction(c, vd)`.
	bean.Validate = func(c echo.Context, vd *validator.Validate) {
		beanValidator.TestUserIdValidation(c, vd)
		
		// Add your own validation function here.
	}

	bean.Bootstrap()
}
