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
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	fpath "path"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	str "github.com/retail-ai-inc/bean/string"
	"github.com/spf13/cobra"
)

var (
	validationRule = `A module path must satisfy the following requirements:

	1. The path must consist of one or more path elements separated by slashes (/, U+002F). It must not begin or end with a slash.
	2. Each path element is a non-empty string made of up ASCII letters, ASCII digits, and limited ASCII punctuation (-, ., _).
	3. A path element may not begin or end with a dot (., U+002E).
	4. The leading path element (up to the first slash, if any), by convention a domain name, must contain only lower-case ASCII letters, ASCII digits, dots (., U+002E), and dashes (-, U+002D); it must contain at least one dot and cannot start with a dash.`

	// initCmd represents the init command
	initCmd = &cobra.Command{
		Use:   "init package_name",
		Short: "Initialize a project in current directory",
		Long: `Init generates all the directories and files structures needed in the current
directory. the suffix of the package_name should match the current directory.`,
		Example: "bean init github.com/retail-ai-inc/bean",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			pkgPath := args[0]
			pkgName, err := getProjectName(pkgPath)
			if err != nil {
				log.Fatalln(validationRule)
			}

			wd, err := os.Getwd()
			if err != nil {
				log.Fatalln(err)
			}

			// Generate a 32 character long random secret string.
			secret, err := str.GenerateRandomString(32, false)
			if err != nil {
				log.Fatalln(err)
			}

			// Generate a 24 character long random JWT secret string.
			jwtSecret, err := str.GenerateRandomString(24, false)
			if err != nil {
				log.Fatalln(err)
			}

			// Generate a 36 character long random token string.
			bearerToken, err := str.GenerateRandomString(36, false)
			if err != nil {
				log.Fatalln(err)
			}

			p := &Project{
				Copyright:   copyright,
				PkgPath:     pkgPath,
				PkgName:     pkgName,
				RootDir:     wd,
				Secret:      secret,
				JWTSecret:   jwtSecret,
				BearerToken: bearerToken,
			}

			// Set the relative root path of the internal FS.
			if p.RootFS, err = fs.Sub(InternalFS, "internal/project"); err != nil {
				log.Fatalln(err)
			}

			fmt.Println("initializing " + p.PkgName + "...")
			if err := fs.WalkDir(p.RootFS, ".", p.generateProjectFiles); err != nil {
				log.Fatalln(err)
			}

			fmt.Println("\ntidying go mod...")
			goModTidyCmd := exec.Command("go", "mod", "tidy")
			goModTidyCmd.Stdout = os.Stdout
			goModTidyCmd.Stderr = os.Stderr
			if err := goModTidyCmd.Run(); err != nil {
				log.Fatalln(err)
			}
		},
	}
)

func init() {
	initCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(initCmd)
}

func getProjectName(pkgPath string) (string, error) {
	validate := validator.New()

	if errs := validate.Var(pkgPath, "required,max=100,startsnotwith=/,endsnotwith=/"); errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			return "", errs
		}
		log.Fatalln(errs)
	}

	pathElements := strings.Split(pkgPath, "/")
	for _, element := range pathElements {
		if errs := validate.Var(element, "required,printascii,excludesall=!\"#$%&'()*+0x2C:;<=>?@[\\]^`{0x7C~},startsnotwith=.,endsnotwith=."); errs != nil {
			if errs, ok := errs.(validator.ValidationErrors); ok {
				return "", errs
			}
			log.Fatalln(errs)
		}
	}

	if len(pathElements) > 1 {
		domain := pathElements[0]
		if errs := validate.Var(domain, "required,max=100,fqdn"); errs != nil {
			if errs, ok := errs.(validator.ValidationErrors); ok {
				return "", errs
			}
			log.Fatalln(errs)
		}
	}

	pkgName := pathElements[len(pathElements)-1]

	return pkgName, nil
}

func (p *Project) generateProjectFiles(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if d.IsDir() {
		// Create the same directory under current directory.
		if err := os.Mkdir(p.RootDir+"/"+path, 0754); err != nil && d.Name() != "project" {
			return err
		}
	} else {
		// Create templates from the files.
		var temp *template.Template
		fmt.Println(p.RootDir + "/" + path)

		if strings.HasSuffix(path, ".html") {
			// For .html files, just copy and not need to pass the template.
			file, err := p.RootFS.Open(path)
			if err != nil {
				return err
			}

			content, err := ioutil.ReadAll(file)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(p.RootDir+"/"+path, content, 0664)
			if err != nil {
				return err
			}

			return nil

		} else {
			// No preprocess for other files.
			temp, err = template.ParseFS(p.RootFS, path)
			if err != nil {
				return err
			}
		}

		// For files which start with `.`, for example `.gitignore`.
		fileName := path
		fileNameWithoutPath := fpath.Base(path)
		if strings.HasPrefix(fileNameWithoutPath, "bean-dot") {
			parentPath := strings.TrimSuffix(path, fileNameWithoutPath)
			fileName = parentPath + strings.TrimPrefix(fileNameWithoutPath, "bean-dot")
		}

		// Strip off the `.tpl` file
		fileName = strings.TrimSuffix(fileName, ".tpl")

		file, err := os.Create(p.RootDir + "/" + fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		// Parse the template and write to the files.
		fileTemplate := template.Must(temp, err)
		err = fileTemplate.Execute(file, p)
		if err != nil {
			return err
		}
	}

	return nil
}
