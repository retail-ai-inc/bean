/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package handlers

import (
	"net/http"

	/**#bean*/
	"demo/framework/internals/async"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/async")**/
	/**#bean*/
	"demo/services"
	/*#bean.replace("{{ .PkgPath }}/services")**/

	"github.com/labstack/echo/v4"
)

type myTestHandler struct {
	myTestService services.MyTestService
}

func NewMyTestHandler(myTestSvc services.MyTestService) *myTestHandler {
	return &myTestHandler{myTestSvc}
}

func (handler *myTestHandler) MyTestIndex(c echo.Context) error {

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
