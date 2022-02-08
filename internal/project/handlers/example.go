/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package handlers

import (
	"net/http"
	"time"

	/**#bean*/
	"demo/framework/internals/async"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/async")**/
	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	/**#bean*/
	"demo/services"
	/*#bean.replace("{{ .PkgPath }}/services")**/

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
)

type ExampleHandler interface {
	JSONIndex(c echo.Context) error // Test JSON index page
	HTMLIndex(c echo.Context) error // Test HTML index page
}

type exampleHandler struct {
	exampleService services.ExampleService
}

func NewExampleHandler(exampleSvc services.ExampleService) *exampleHandler {
	return &exampleHandler{exampleSvc}
}

func (handler *exampleHandler) JSONIndex(c echo.Context) error {
	span := sentry.StartSpan(c.Request().Context(), "handler")
	span.Description = helpers.CurrFuncName()
	defer span.Finish()

	dbName, err := handler.exampleService.GetMasterSQLTableList(span.Context())
	if err != nil {
		return err
	}

	// IMPORTANT: This is how you can execute some asynchronous code instead `go routine`.
	async.Execute(func(c echo.Context) {
		c.Logger().Debug(dbName)
	}, c.Echo())

	return c.JSON(http.StatusOK, map[string]string{
		"dbName": dbName,
	})
}

func (handler *exampleHandler) HTMLIndex(c echo.Context) error {

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
