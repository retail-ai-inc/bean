// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
)

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
directory. the suffix of the package_name should match the current directory.`,
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

			p := &Project{
				Copyright: `// Copyright The RAI Inc.
// The RAI Authors`,
				PkgName:     pkgName,
				PrjName:     prjName,
				RootDir:     wd,
				BeanVersion: rootCmd.Version,
			}

			// Set the relative root path of the internal FS.
			if p.RootFS, err = fs.Sub(InternalFS, "internal/project"); err != nil {
				log.Fatalln(err)
			}

			fmt.Println("initializing " + p.PrjName + "...")
			if err := fs.WalkDir(p.RootFS, ".", p.generateProjectFiles); err != nil {
				log.Fatalln(err)
			}

			fmt.Println("\ninitializing go mod...")
			goModInitCmd := exec.Command("go", "mod", "init", p.PkgName)
			goModInitCmd.Stdout = os.Stdout
			goModInitCmd.Stderr = os.Stderr
			if err := goModInitCmd.Run(); err != nil {
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

func validate(projName string) {
	validate := validator.New()
	errs := validate.Var(projName, "required,alphanum")
	if errs != nil {
		log.Fatalln(validationRule)
	}
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
		// Create the files.
		fileName := strings.TrimSuffix(path, ".tpl")
		fmt.Println(fileName)
		file, err := os.Create(p.RootDir + "/" + fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		if fileName == "bean.sh" {
			if err := file.Chmod(0755); err != nil {
				return err
			}
		}

		// Parse the template and write to the files.
		fileTemplate := template.Must(template.ParseFS(p.RootFS, path))
		err = fileTemplate.Execute(file, p)
		if err != nil {
			return err
		}
	}

	return nil
}
