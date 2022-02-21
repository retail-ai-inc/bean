{{ .Copyright }}
package handlers

import (
	"net/http"
	"time"

	"{{ .PkgPath }}/services"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/framework/async"
	berror "github.com/retail-ai-inc/bean/framework/error"
	"github.com/retail-ai-inc/bean/framework/helpers"
)

type ExampleHandler interface {
	JSONIndex(c echo.Context) error // JSON example
	HTMLIndex(c echo.Context) error // HTML example
	Validate(c echo.Context) error  // Validation example
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

	// IMPORTANT: Panic inside a goroutine will crash the whole application.
	// Example: How to execute some asynchronous code safely instead of plain goroutine.
	async.Execute(func(c echo.Context) {
		c.Logger().Debug(dbName)
		// IMPORTANT: Using sentry directly in goroutine may cause data race!
		// Need to create a new hub by cloning the existing one.
		// Example: How to use sentry safely in goroutine.
		// localHub := sentry.CurrentHub().Clone()
		// localHub.CaptureMessage(dbName)
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

func (handler *exampleHandler) Validate(c echo.Context) error {
	var params struct {
		Example string `json:"example" validate:"required,example,min=7"`
		Other   int    `json:"other" validate:"required,gt=0"`
	}

	if err := c.Bind(&params); err != nil {
		return berror.NewAPIError(http.StatusBadRequest, berror.PROBLEM_PARSING_JSON, err)
	}

	if err := c.Validate(params); err != nil {
		return err
	}

	return c.String(http.StatusOK, params.Example+" OK\n")
}
