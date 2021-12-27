{{ .Copyright }}
package routers

import (
	"{{ .PkgName }}/framework/internals/global"
	"{{ .PkgName }}/handlers"
	"{{ .PkgName }}/repositories"
	"{{ .PkgName }}/services"

	"github.com/labstack/echo/v4"
)

type Repositories struct {
	MyTestRepo repositories.MyTestRepository
}

type Services struct {
	MyTestSvc services.MyTestService
}

type Handlers struct {
	MyTestHdlr handlers.MyTestHandler
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