/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package routers

import (
	/**#bean*/
	"demo/framework/internals/global"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/global")**/
	/**#bean*/
	"demo/handlers"
	/*#bean.replace("{{ .PkgPath }}/handlers")**/
	/**#bean*/
	"demo/repositories"
	/*#bean.replace("{{ .PkgPath }}/repositories")**/
	/**#bean*/
	"demo/services"
	/*#bean.replace("{{ .PkgPath }}/services")**/)

type Repositories struct {
	MyTestRepo repositories.MyTestRepository
}

type Services struct {
	MyTestSvc services.MyTestService
}

type Handlers struct {
	MyTestHdlr handlers.MyTestHandler
}

func Init() {

	e := global.EchoInstance

	repos := &Repositories{
		MyTestRepo: repositories.NewMyTestRepository(global.DBConn),
	}

	svcs := &Services{
		MyTestSvc: services.NewMyTestService(repos.MyTestRepo),
	}

	hdlrs := &Handlers{
		MyTestHdlr: handlers.NewMyTestHandler(svcs.MyTestSvc),
	}

	// IMPORTANT: Just a JSON response index page. Please change or update it if you want.
	e.GET("/json", hdlrs.MyTestHdlr.MyTestJSONIndex)

	// IMPORTANT: Just a HTML response index page. Please change or update it if you want.
	e.GET("/html", hdlrs.MyTestHdlr.MyTestHTMLIndex)

	// Default index page goes to above JSON (/json) index page.
	e.GET("/", hdlrs.MyTestHdlr.MyTestJSONIndex)
}
