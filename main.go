/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package main

import (
	_ "bean/commands"
	"bean/framework"
	"bean/framework/internals/validator"
	"bean/routers"
)

func main() {

	bean := new(framework.Bean)
	bean.Router = routers.Init
	bean.MiddlewareInitializer = nil
	bean.Validate = validator.User4ubarcodeid

	bean.Bootstrap()
}
