{{ .Copyright }}
package main

import (
	"{{ .PkgName }}/framework"
	"{{ .PkgName }}/framework/internals/validator"
	"{{ .PkgName }}/routers"
)

func main() {

	bean := new(framework.Bean)
	bean.Router = routers.Init
	bean.MiddlewareInitializer = nil
	bean.Validate = validator.TestUserIdValidation

	bean.Bootstrap()
}
