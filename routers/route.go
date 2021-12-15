/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package routers

import (
	"bean/framework/internals/global"
	"bean/handlers"
	"bean/interfaces"
	"bean/repositories"
	"bean/services"

	"github.com/labstack/echo/v4"
)

type Repositories struct {
	MyTestRepo interfaces.MyTestRepository
}

type Services struct {
	MyTestSvc interfaces.MyTestService
}

type Handlers struct {
	MyTestHdlr interfaces.MyTestHandler
}

func Init(e *echo.Echo) {

	repos := &Repositories{
		MyTestRepo: repositories.NewMyTestRepository(global.DBConn),
	}

	svcs := &Services{
		MyTestSvc: services.NewMyTestService(repos.MyTestRepo),
	}

	hdlrs := &Handlers{
		MyTestHdlr: handlers.NewMyTestHandler(svcs.MyTestSvc),
	}

	// Just a index page.
	e.GET("/", hdlrs.MyTestHdlr.MyTestIndex)
}
