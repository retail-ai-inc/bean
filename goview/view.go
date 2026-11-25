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
	tplMap      map[string]*cachedTemplate
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
		tplMap:      make(map[string]*cachedTemplate),
		tplMutex:    sync.RWMutex{},
		fileHandler: DefaultFileHandler(),
	}
}

// Default new default template engine
func Default() *ViewEngine {

	return New(DefaultConfig)
}

// Render render template with http.ResponseWriter. The template is rendered into a buffer first, so a template
// error returns an error without writing a partial response or a status code.
func (e *ViewEngine) Render(w http.ResponseWriter, statusCode int, name string, data interface{}) error {
	var buf bytes.Buffer
	if err := e.executeRender(&buf, name, data); err != nil {
		return err
	}

	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = HTMLContentType
	}
	w.WriteHeader(statusCode)
	_, err := buf.WriteTo(w)
	return err
}

// RenderWriter render template with io.Writer
func (e *ViewEngine) RenderWriter(w io.Writer, name string, data interface{}) error {

	return e.executeRender(w, name, data)
}

// executeRender renders the template `name`. The layout (master) is chosen per render from `data["template"]`:
//   - a non-empty string: that layout,
//   - an empty string: `Config.Master` if its file exists,
//   - absent: no layout.
//
// The engine is safe for concurrent use: the layout is never stored in the shared configuration.
func (e *ViewEngine) executeRender(out io.Writer, name string, data interface{}) error {
	var dataMap map[string]any
	switch v := data.(type) {
	case map[string]any:
		dataMap = v
	case M:
		dataMap = v
	default:
		return fmt.Errorf("ViewEngine invalid data type in `Render` function: %v", v)
	}

	var master string
	if layout, ok := dataMap["template"].(string); ok {
		if layout != "" {
			master = layout
		} else if e.config.Master != "" {
			// Use the master template if the file exists.
			if _, err := os.Stat(e.config.Root + "/" + e.config.Master + e.config.Extension); !errors.Is(err, os.ErrNotExist) {
				master = e.config.Master
			}
		}
	}

	return e.executeTemplate(out, name, data, master)
}

// executeTemplate renders the template `name` within the layout `master` (none if empty).
//
// The `include` function must render with the data of this render, so it is bound to each render. Changing the
// functions of a template shared by concurrent renders is a data race and lets a render `include` with the data of
// another request, so each render executes a template that no other render uses at the same time: a fresh parse when
// the cache is disabled, otherwise a clone of the cached parse taken from a pool (cloning on every render would copy
// all the parse trees each time).
func (e *ViewEngine) executeTemplate(out io.Writer, name string, data interface{}, master string) error {
	tpl, release, err := e.acquireTemplate(name, master)
	if err != nil {
		return err
	}
	defer release()

	tpl.Funcs(template.FuncMap{
		"include": func(inclTmpl string) (template.HTML, error) {
			var buf bytes.Buffer
			err := e.executeTemplate(&buf, inclTmpl, data, "")
			return template.HTML(buf.String()), err
		},
	})

	exeName := name
	if master != "" {
		exeName = master
	}
	if err := tpl.ExecuteTemplate(out, exeName, data); err != nil {
		return fmt.Errorf("ViewEngine execute template error: %v", err)
	}

	return nil
}

// cachedTemplate is a parsed template set (never executed) and a pool of its clones, which are executed.
type cachedTemplate struct {
	parsed *template.Template
	clones sync.Pool
}

// acquireTemplate returns a template for `name` within the layout `master` for the exclusive use of the caller,
// who must call release once the template is executed.
func (e *ViewEngine) acquireTemplate(name, master string) (tpl *template.Template, release func(), err error) {
	if e.config.DisableCache {
		tpl, err = e.parseTemplate(name, master)
		return tpl, func() {}, err
	}

	// The parsed set depends on the layout, so the layout is part of the cache key.
	key := master + "\x00" + name

	e.tplMutex.RLock()
	cached, ok := e.tplMap[key]
	e.tplMutex.RUnlock()
	if !ok {
		parsed, err := e.parseTemplate(name, master)
		if err != nil {
			return nil, nil, err
		}
		e.tplMutex.Lock()
		if cached, ok = e.tplMap[key]; !ok {
			cached = &cachedTemplate{parsed: parsed}
			e.tplMap[key] = cached
		}
		e.tplMutex.Unlock()
	}

	if v := cached.clones.Get(); v != nil {
		tpl = v.(*template.Template)
	} else if tpl, err = cached.parsed.Clone(); err != nil {
		return nil, nil, fmt.Errorf("ViewEngine clone template name:%v, error: %v", name, err)
	}
	return tpl, func() {
		// Drop the reference to the data of this render before pooling the clone.
		tpl.Funcs(template.FuncMap{"include": includePlaceholder})
		cached.clones.Put(tpl)
	}, nil
}

// includePlaceholder lets templates using `include` parse; it is replaced on each render, see `executeTemplate`.
func includePlaceholder(string) (template.HTML, error) { return "", nil }

// parseTemplate parses the template `name` with the layout `master` and the partials.
func (e *ViewEngine) parseTemplate(name, master string) (*template.Template, error) {
	funcs := make(template.FuncMap, len(e.config.Funcs)+1)
	funcs["include"] = includePlaceholder
	for k, v := range e.config.Funcs {
		funcs[k] = v
	}

	tplList := make([]string, 0, len(e.config.Partials)+2)
	if master != "" {
		tplList = append(tplList, master)
	}
	tplList = append(tplList, name)
	tplList = append(tplList, e.config.Partials...)

	tpl := template.New(name).Funcs(funcs).Delims(e.config.Delims.Left, e.config.Delims.Right)
	for _, v := range tplList {
		content, err := e.fileHandler(e.config, v)
		if err != nil {
			return nil, err
		}

		tmpl := tpl
		if v != name {
			tmpl = tpl.New(v)
		}
		if _, err := tmpl.Parse(content); err != nil {
			return nil, fmt.Errorf("ViewEngine render parser name:%v, error: %v", v, err)
		}
	}
	return tpl, nil
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
