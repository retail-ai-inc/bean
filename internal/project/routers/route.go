/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package routers

import (
	/**#bean*/ "demo/framework/internals/global" /*#bean.replace("{{ .PkgPath }}/framework/internals/global")**/
	/**#bean*/ "demo/handlers" /*#bean.replace("{{ .PkgPath }}/handlers")**/
	/**#bean*/ "demo/repositories" /*#bean.replace("{{ .PkgPath }}/repositories")**/
	/**#bean*/ "demo/services" /*#bean.replace("{{ .PkgPath }}/services")**/

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
