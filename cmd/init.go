// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
)

// Project contains name, license and paths to projects.
type Project struct {
	// Copyright is the copyright text on the top of every files.
	Copyright string
	// PkgName is the full string of the generated package (example: github.com/retail-ai-inc/bean).
	PkgName string
	// PrjName is the suffix of the package name, it should match the current directory name.
	PrjName string
	// AbsolutePath is the current working directory path when executing the bean command.
	AbsolutePath string
	// BeanVersion is the current bean version.
	BeanVersion string
}

var (
	validationRule = `1. lowercase-letters (a-z0-9) (that may separated by underscores _). Names may not contain spaces or any other special characters.
2. A project name can not start with 0-9 or underscore
3. Cannot use these reserved keywords: break, default, func, interface, select, case, defer, go, map, struct, chan, else, goto, package, switch, const, fallthrough, if, range, type, continue, for, import, return, var.
4. Cannot use these reserved keywords either: append, bool, byte, cap, close, complex, complex64, complex128, uint16, copy, false, float32, float64, imag, int, int8, int16, uint32, int32, int64, iota, len, make, new, nil, panic, uint64, print, println, real, recover, string, true, uint, uint8, uintptr,`

	// initCmd represents the init command
	initCmd = &cobra.Command{
		Use:   "init package_name",
		Short: "Initialize a project in current directory",
		Long: `Init generates all the directories and files structures needed in the current
directory. the suffix of the packagename should match the current directory.`,
		Example: `bean init github.com/retail-ai-inc/bean`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			pkgName := args[0]
			// TODO: validate(pkgName)
			s := strings.Split(pkgName, "/")
			prjName := s[len(s)-1]
			wd, err := os.Getwd()
			if err != nil {
				log.Fatalln(err)
			}

			project := &Project{
				Copyright:    "// Copyright The RAI Inc." + "/n" + "// The RAI Authors",
				PkgName:      pkgName,
				AbsolutePath: wd,
				PrjName:      prjName,
				BeanVersion:  Version,
			}

			fmt.Println("initializing " + prjName + "...")
			if err := filepath.WalkDir(ModulePath+"/internal", project.generateFiles); err != nil {
				log.Fatalln(err)
			}

			fmt.Println("initializing go mod...")
			if err := exec.Command("go", "mod", "init", pkgName).Run(); err != nil {
				log.Fatalln(err)
			}

			fmt.Println("tidying go mod...")
			if err := exec.Command("go", "mod", "tidy").Run(); err != nil {
				log.Fatalln(err)
			}

		},
	}
)

func init() {
	initCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(initCmd)
}

func validate(projName string) {
	validate := validator.New()
	errs := validate.Var(projName, "required,alphanum")
	if errs != nil {
		log.Fatalln(validationRule)
	}
}

func (p *Project) generateFiles(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	// skip creating the root directory
	if d.IsDir() && d.Name() == "internal" {
		return nil
	}

	if d.IsDir() {
		// Create the same directory under current directory.
		relativePath := strings.TrimPrefix(path, ModulePath+"/internal")
		if err := os.Mkdir(p.AbsolutePath+relativePath, 0754); err != nil {
			return err
		}
	} else {
		// Create the files.
		relativePath := strings.TrimPrefix(path, ModulePath+"/internal")
		fileName := strings.TrimSuffix(relativePath, ".tpl")
		file, err := os.Create(p.AbsolutePath + fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		if fileName == "/bean.sh" {
			if err := file.Chmod(0755); err != nil {
				return err
			}
		}

		// Parse the template and write to the files.
		fileTemplate := template.Must(template.ParseFiles(path))
		err = fileTemplate.Execute(file, p)
		if err != nil {
			return err
		}
	}

	return nil
}
