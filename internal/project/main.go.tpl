{{ .Copyright }}
package main

import (
	"{{ .PkgPath }}/framework"
	"{{ .PkgPath }}/framework/internals/validator"
	"{{ .PkgPath }}/routers"
)

func main() {

	bean := new(framework.Bean)
	bean.Router = routers.Init
	bean.MiddlewareInitializer = nil
	bean.Validate = validator.TestUserIdValidation

	bean.Bootstrap()
}
