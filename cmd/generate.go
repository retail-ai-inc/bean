// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/go-playground/validator/v10"
)

// errNotBeanProject is returned by the `bean create ...` commands outside a bean project.
var errNotBeanProject = errors.New("env.json not found: run this command in the root directory of a bean project")

// loadProject returns the bean project of the current directory, with the templates of `bean create ...`.
func loadProject() (*Project, error) {
	if _, err := os.Stat("env.json"); errors.Is(err, os.ErrNotExist) {
		return nil, errNotBeanProject
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	p := &Project{Copyright: copyright, RootDir: wd}

	if p.RootFS, err = fs.Sub(InternalFS, "internal/_tpl"); err != nil {
		return nil, err
	}
	if p.PkgPath, err = getPackagePathNameFromEnv(p); err != nil {
		return nil, fmt.Errorf("failed to read packagePath from env.json: %w", err)
	}
	return p, nil
}

// fileExists reports whether the file at the path (relative to dir) exists.
func fileExists(dir string, path ...string) bool {
	_, err := os.Stat(filepath.Join(append([]string{dir}, path...)...))
	return err == nil
}

// componentName validates the name of a generated component (handler, repository, service) and returns it with
// its first letter upper case and the rest lower case, e.g. `post` -> `Post`.
func componentName(kind, name string) (string, error) {
	if err := validateName(kind, name); err != nil {
		return "", err
	}
	return strings.ToUpper(name[:1]) + strings.ToLower(name[1:]), nil
}

// validateName checks the rules of a component / command name: 1 to 100 ASCII letters or digits, not starting
// with a digit.
func validateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("the %s name is empty", kind)
	}
	if unicode.IsDigit(rune(name[0])) {
		return fmt.Errorf("the %s name %q must not start with a digit", kind, name)
	}
	if err := validator.New().Var(name, "required,max=100,alphanum"); err != nil {
		return fmt.Errorf("the %s name %q must be 1 to 100 ASCII letters or digits", kind, name)
	}
	return nil
}

// generateFile renders the template tpl of the project's templates to relPath (relative to the project root).
// A Go file is gofmt'ed. It fails if the file already exists.
func generateFile(p *Project, tpl, relPath string, data any) error {
	path := filepath.Join(p.RootDir, relPath)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", relPath)
	}

	content, err := renderTemplate(p.RootFS, tpl, data)
	if err != nil {
		return err
	}
	if strings.HasSuffix(relPath, ".go") {
		if content, err = format.Source(content); err != nil {
			return fmt.Errorf("generated %s is not valid Go: %w", relPath, err)
		}
	}
	return os.WriteFile(path, content, 0o644)
}

// renderTemplate executes the template tpl of fsys with data.
func renderTemplate(fsys fs.FS, tpl string, data any) ([]byte, error) {
	src, err := fs.ReadFile(fsys, tpl)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(tpl).Parse(string(src))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
