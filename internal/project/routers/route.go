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
	MyTestRepo repositories.MyTestRepository
}

type Services struct {
	MyTestSvc services.MyTestService
}

func Init(b *bean.Bean) {

	e := b.Echo

	repos := &Repositories{
		MyTestRepo: repositories.NewMyTestRepository(b.DBConn),
	}

	svcs := &Services{
		MyTestSvc: services.NewMyTestService(repos.MyTestRepo),
	}

	myTestHandler := handlers.NewMyTestHandler(svcs.MyTestSvc)

	// TODO: Maybe don't need this neither.
	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": `{{ .PkgName }} 🚀  pong`,
		})
	})

	// Just a index page.
	e.GET("/", myTestHandler.MyTestIndex)
}
