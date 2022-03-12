{{ .ProjectObject.Copyright }}
package handlers

import (
	"net/http"
	"time"
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/async"
	berror "github.com/retail-ai-inc/bean/error"
	"github.com/retail-ai-inc/bean/helpers"
	{{if .ServiceExists}}"{{.ProjectObject.PkgPath}}/services"{{end}}
)

type {{.HandlerNameUpper}}Handler interface {
	JSONIndex(c echo.Context) error // JSON {{.HandlerNameLower}}
	HTMLIndex(c echo.Context) error // HTML {{.HandlerNameLower}}
	Validate(c echo.Context) error  // Validation {{.HandlerNameLower}}
}

{{if .ServiceExists}}
type {{.HandlerNameLower}}Handler struct {
	{{.HandlerNameLower}}Service services.{{.HandlerNameUpper}}Service
}{{else}}type {{.HandlerNameLower}}Handler struct {}{{end}}


{{if .ServiceExists}}func New{{.HandlerNameUpper}}Handler({{.HandlerNameLower}}Svc services.{{.HandlerNameUpper}}Service) *{{.HandlerNameLower}}Handler {
	return &{{.HandlerNameLower}}Handler{{"{"}}{{.HandlerNameLower}}Svc{{"}"}}
}{{else}}func New{{.HandlerNameUpper}}Handler() *{{.HandlerNameLower}}Handler {
	return &{{.HandlerNameLower}}Handler{{"{}}"}}{{end}}

func (handler *{{.HandlerNameLower}}Handler) JSONIndex(c echo.Context) error {
	span := sentry.StartSpan(c.Request().Context(), "http.handler")
	span.Description = helpers.CurrFuncName()
	defer span.Finish()

	{{if .ServiceExists}}output, err := handler.{{.HandlerNameLower}}Service.{{.HandlerNameUpper}}ServiceExampleFunc(span.Context())
	if err != nil {
		return err
	}{{else}}output := "output"{{end}}

	// IMPORTANT: Panic inside a goroutine will crash the whole application.
	// Example: How to execute some asynchronous code safely instead of plain goroutine:
	async.Execute(func(c echo.Context) {
		c.Logger().Debug(output)
		// IMPORTANT: Using sentry directly in goroutine may cause data race!
		// Need to create a new hub by cloning the existing one.
		// Example: How to use sentry safely in goroutine.
		// localHub := sentry.CurrentHub().Clone()
		// localHub.CaptureMessage(output)
	}, c.Echo())

	return c.JSON(http.StatusOK, map[string]string{
		"output": output,
	})
}

func (handler *{{.HandlerNameLower}}Handler) HTMLIndex(c echo.Context) error {
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

func (handler *{{.HandlerNameLower}}Handler) Validate(c echo.Context) error {
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
