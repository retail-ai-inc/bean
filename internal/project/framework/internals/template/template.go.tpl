{{ .Copyright }}
// Don't named same for multiple template files even in different directories, otherwise this wrapper will
// load only the last file with same name.
package template

import (
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

type Template struct {
	templates *template.Template
}

func New(e *echo.Echo) *Template {

	return &Template{

		templates: func() *template.Template {

			templ := template.New("")

			if err := filepath.Walk("views", func(path string, info os.FileInfo, err error) error {

				if strings.Contains(path, ".html") {

					_, err = templ.ParseFiles(path)

					if err != nil {

						e.Logger.Error(err)
					}
				}

				return err

			}); err != nil {

				e.Logger.Fatal(err)
			}

			return templ
		}(),
	}
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {

	if err := t.templates.ExecuteTemplate(w, name, data); err != nil {

		return err
	}

	return nil
}
