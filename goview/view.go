// The MIT License (MIT)

// Copyright (c) 2018 Foolin

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package goview

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/labstack/echo/v4"
)

// HTMLContentType const templateEngineKey = "httpx_templateEngine"
var HTMLContentType = []string{"text/html; charset=utf-8"}

// DefaultConfig default config
var DefaultConfig = Config{
	Root:         "views",
	Extension:    ".html",
	Master:       "templates/master",
	Partials:     []string{},
	Funcs:        make(template.FuncMap),
	DisableCache: false,
	Delims:       Delims{Left: "{{", Right: "}}"},
}

// ViewEngine view template engine
type ViewEngine struct {
	config      Config
	tplMap      map[string]*template.Template
	tplMutex    sync.RWMutex
	fileHandler FileHandler
}

// Config configuration options
type Config struct {
	Root         string           //view root
	Extension    string           //template extension
	Master       string           //template master
	Partials     []string         //template partial, such as head, foot
	Funcs        template.FuncMap //template functions
	DisableCache bool             //disable cache, debug mode
	Delims       Delims           //delimeters
}

// M map interface for data
type M map[string]interface{}

// Delims delims for template
type Delims struct {
	Left  string
	Right string
}

// FileHandler file handler interface
type FileHandler func(config Config, tplFile string) (content string, err error)

// New new template engine
func New(config Config) *ViewEngine {
	return &ViewEngine{
		config:      config,
		tplMap:      make(map[string]*template.Template),
		tplMutex:    sync.RWMutex{},
		fileHandler: DefaultFileHandler(),
	}
}

// Default new default template engine
func Default() *ViewEngine {

	return New(DefaultConfig)
}

// Render render template with http.ResponseWriter
func (e *ViewEngine) Render(w http.ResponseWriter, statusCode int, name string, data interface{}) error {

	header := w.Header()

	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = HTMLContentType
	}

	w.WriteHeader(statusCode)
	return e.executeRender(w, name, data)
}

// RenderWriter render template with io.Writer
func (e *ViewEngine) RenderWriter(w io.Writer, name string, data interface{}) error {

	return e.executeRender(w, name, data)
}

func (e *ViewEngine) executeRender(out io.Writer, name string, data interface{}) error {

	// If user doesn't set the template then useMaster = false means no template.
	var useMaster bool
	var dataMap map[string]interface{}

	switch v := data.(type) {
	case echo.Map:
		dataMap = v
	case map[string]interface{}:
		dataMap = v
	default:
		return fmt.Errorf("ViewEngine invalid data type in `Render` function: %v", v)
	}

	// If user provide a template layout then use that instead `templates/master.html`
	if template, ok := dataMap["template"].(string); ok {
		if template != "" {
			useMaster = true
			e.config.Master = template

		} else {
			// Try to use master template if the file exist otherwise set useMaster = false
			if _, err := os.Stat(e.config.Root + "/" + e.config.Master + e.config.Extension); errors.Is(err, os.ErrNotExist) {
				useMaster = false
			} else {
				useMaster = true
			}
		}
	}

	return e.executeTemplate(out, name, data, useMaster)
}

func (e *ViewEngine) executeTemplate(out io.Writer, name string, data interface{}, useMaster bool) error {

	var tpl *template.Template
	var err error
	var ok bool

	allFuncs := make(template.FuncMap)
	allFuncs["include"] = func(inclTmpl string) (template.HTML, error) {
		buf := new(bytes.Buffer)
		err := e.executeTemplate(buf, inclTmpl, data, false)
		return template.HTML(buf.String()), err
	}

	// Get the plugin collection
	for k, v := range e.config.Funcs {
		allFuncs[k] = v
	}

	e.tplMutex.RLock()
	tpl, ok = e.tplMap[name]
	e.tplMutex.RUnlock()

	exeName := name

	if useMaster && e.config.Master != "" {
		exeName = e.config.Master
	}

	if !ok || e.config.DisableCache {
		tplList := make([]string, 0)

		if useMaster {
			//render()
			if e.config.Master != "" {
				tplList = append(tplList, e.config.Master)
			}
		}

		tplList = append(tplList, name)
		tplList = append(tplList, e.config.Partials...)

		// Loop through each template and test the full path
		tpl = template.New(name).Funcs(allFuncs).Delims(e.config.Delims.Left, e.config.Delims.Right)

		for _, v := range tplList {
			var data string
			data, err = e.fileHandler(e.config, v)

			if err != nil {
				return err
			}

			var tmpl *template.Template

			if v == name {
				tmpl = tpl
			} else {
				tmpl = tpl.New(v)
			}

			_, err = tmpl.Parse(data)
			if err != nil {
				return fmt.Errorf("ViewEngine render parser name:%v, error: %v", v, err)
			}
		}

		e.tplMutex.Lock()
		e.tplMap[name] = tpl
		e.tplMutex.Unlock()
	}

	// Display the content to the screen
	err = tpl.Funcs(allFuncs).ExecuteTemplate(out, exeName, data)
	if err != nil {
		return fmt.Errorf("ViewEngine execute template error: %v", err)
	}

	return nil
}

// SetFileHandler set file handler
func (e *ViewEngine) SetFileHandler(handle FileHandler) {

	if handle == nil {
		panic("FileHandler can't set nil!")
	}

	e.fileHandler = handle
}

// DefaultFileHandler new default file handler
func DefaultFileHandler() FileHandler {

	return func(config Config, tplFile string) (content string, err error) {
		// Get the absolute path of the root template
		path, err := filepath.Abs(config.Root + string(os.PathSeparator) + tplFile + config.Extension)
		if err != nil {
			return "", fmt.Errorf("ViewEngine path:%v error: %v", path, err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("ViewEngine render read name:%v, path:%v, error: %v", tplFile, path, err)
		}

		return string(data), nil
	}
}
