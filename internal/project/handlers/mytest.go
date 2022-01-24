/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package handlers

import (
	"net/http"
	"time"

	/**#bean*/
	"demo/framework/internals/async"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/async")**/
	/**#bean*/
	"demo/services"
	/*#bean.replace("{{ .PkgPath }}/services")**/

	"github.com/labstack/echo/v4"
)

type MyTestHandler interface {
	MyTestJSONIndex(c echo.Context) error // Test JSON index page
	MyTestHTMLIndex(c echo.Context) error // Test HTML index page
}

type myTestHandler struct {
	myTestService services.MyTestService
}

func NewMyTestHandler(myTestSvc services.MyTestService) *myTestHandler {
	return &myTestHandler{myTestSvc}
}

func (handler *myTestHandler) MyTestJSONIndex(c echo.Context) error {

	dbName, err := handler.myTestService.GetMasterSQLTableList(c)
	if err != nil {
		return err
	}

	async.Execute(func(c echo.Context) {
		c.Logger().Debug(dbName)
	}, c.Echo())

	return c.JSON(http.StatusOK, map[string]string{
		"dbName": dbName,
	})
}

func (handler *myTestHandler) MyTestHTMLIndex(c echo.Context) error {

	return c.Render(http.StatusOK, "index", echo.Map{
		"title": "Index title!",
		"add": func(a int, b int) int {
			return a + b
		},
		"test": map[string]interface{}{
			"a": "hi",
			"b": 10,
		},
		"copyrightYear": time.Now().Year(),
		"template":      "templates/master",
	})
}
