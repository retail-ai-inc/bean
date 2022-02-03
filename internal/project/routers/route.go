/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package routers

import (
	"net/http"

	/**#bean*/
	"demo/framework/bean"
	/*#bean.replace("{{ .PkgPath }}/framework/bean")**/
	/**#bean*/
	"demo/handlers"
	/*#bean.replace("{{ .PkgPath }}/handlers")**/
	/**#bean*/
	"demo/repositories"
	/*#bean.replace("{{ .PkgPath }}/repositories")**/
	/**#bean*/
	"demo/services"
	/*#bean.replace("{{ .PkgPath }}/services")**/

	"github.com/labstack/echo/v4"
)

type Repositories struct {
	exampleRepo repositories.ExampleRepository
}

type Services struct {
	exampleSvc services.ExampleService
}

type Handlers struct {
	exampleHdlr handlers.ExampleHandler
}

func Init(b *bean.Bean) {

	e := b.Echo

	repos := &Repositories{
		exampleRepo: repositories.NewExampleRepository(b.DBConn),
	}

	svcs := &Services{
		exampleSvc: services.NewExampleService(repos.exampleRepo),
	}

	hdlrs := &Handlers{
		exampleHdlr: handlers.NewExampleHandler(svcs.exampleSvc),
	}

	// Default index page goes to above JSON (/json) index page.
	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": `{{ .PkgName }} 🚀`,
		})
	})

	// IMPORTANT: Just a JSON response index page. Please change or update it if you want.
	e.GET("/json", hdlrs.exampleHdlr.JSONIndex)

	// IMPORTANT: Just a HTML response index page. Please change or update it if you want.
	e.GET("/html", hdlrs.exampleHdlr.HTMLIndex)
}
