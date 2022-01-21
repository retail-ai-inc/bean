/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package handlers

import (
	"net/http"
	"time"

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

	res, _ := handler.myTestService.GetMasterSQLTableList(c)

	// IMPORTANT: This is how you can log something.
	c.Logger().Info(res["dbName"])

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Howdy! I am {{ .PkgName }} 🚀 ",
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
