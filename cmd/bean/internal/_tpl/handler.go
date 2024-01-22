package handlers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/async"
	berror "github.com/retail-ai-inc/bean/v2/error"
	"github.com/retail-ai-inc/bean/v2/trace"
	{{if .ServiceExists}}"{{.ProjectObject.PkgPath}}/services"{{end}}
)

type {{.HandlerNameUpper}}Handler interface {
	{{.HandlerNameUpper}}JSONResponse(c echo.Context) error		// An example JSON response handler function
	{{.HandlerNameUpper}}HTMLResponse(c echo.Context) error		// An example HTML response handler function
	{{.HandlerNameUpper}}ValidateResponse(c echo.Context) error	// An example response handler function with validation
}

{{if .ServiceExists}}
type {{.HandlerNameLower}}Handler struct {
	{{.HandlerNameLower}}Service services.{{.HandlerNameUpper}}Service
}{{else}}type {{.HandlerNameLower}}Handler struct {}{{end}}


{{if .ServiceExists}}func New{{.HandlerNameUpper}}Handler({{.HandlerNameLower}}Svc services.{{.HandlerNameUpper}}Service) *{{.HandlerNameLower}}Handler {
	return &{{.HandlerNameLower}}Handler{{"{"}}{{.HandlerNameLower}}Svc{{"}"}}
}{{else}}func New{{.HandlerNameUpper}}Handler() *{{.HandlerNameLower}}Handler {
	return &{{.HandlerNameLower}}Handler{{"{}\n}"}}{{end}}

func (handler *{{.HandlerNameLower}}Handler) {{.HandlerNameUpper}}JSONResponse(c echo.Context) error {
	
	// IMPORTANT: If you wanna trace the performance of your handler function then uncomment following 3 lines
	// tctx := trace.NewTraceableContext(c.Request().Context())
	// finish := trace.Start(tctx, "http.handler")
	// defer finish()

	{{if .ServiceExists}}output, err := handler.{{.HandlerNameLower}}Service.{{.HandlerNameUpper}}ServiceExampleFunc(c.Request().Context())
	if err != nil {
		return err
	}{{else}}output := "output"{{end}}

	// IMPORTANT: Panic inside a goroutine will crash the whole application.
	// Example: How to execute some asynchronous code safely instead of plain goroutine:
	// !!! Do not use the original echo context in the goroutine since it will be released after the handler returned.
	// !!! https://github.com/labstack/echo/issues/1633
	// However there is a recoverPanic function inside the ExecuteWithContext that will
	// prevent this from happening and not crash the app.
	async.ExecuteWithContext(func(asyncC context.Context) {
		c.Logger().Debug(output)
		traceableContext := trace.NewTraceableContext(asyncC)
		asyncFinish := trace.Start(traceableContext, "http.async")
		defer asyncFinish()

		// example function that you want to execute asynchronously.
		files, err := os.ReadDir("./")
		for _, f := range files {
            fmt.Println(f.Name())
		}
		// handling error
		if err != nil {
			fmt.Println(err)
			// This is a global function to send sentry exception if you configure the sentry through env.json. 
			// You cann pass a proper context or nil.
			// async.CaptureException(traceableContext, err)
		}

	},c)

	return c.JSON(http.StatusOK, map[string]string{
		"output": output,
	})
}

func (handler *{{.HandlerNameLower}}Handler) {{.HandlerNameUpper}}HTMLResponse(c echo.Context) error {
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

func (handler *{{.HandlerNameLower}}Handler) {{.HandlerNameUpper}}ValidateResponse(c echo.Context) error {
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
