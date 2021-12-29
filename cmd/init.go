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
	validationRule = `The package should follows the following rule (excluding URL part):
	1. Max length: 100 code points.
	2. ASCII alphanumeric code point including hyphen (-), underscore (_), period (.).
	3. Not start with hyphen (-), an underscore (_), a period (.).`

	// initCmd represents the init command
	initCmd = &cobra.Command{
		Use:   "init package_name",
		Short: "Initialize a project in current directory",
		Long: `Init generates all the directories and files structures needed in the current
directory. the suffix of the package_name should match the current directory.`,
		Example: "bean init github.com/retail-ai-inc/bean",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			pkgName := args[0]
			prjName, err := getProjectName(pkgName)
			if err != nil {
				log.Fatalln(validationRule)
			}

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

func getProjectName(pkgName string) (string, error) {
	validate := validator.New()

	errs := validate.Var(pkgName, "max=100,printascii,excludesall=!\"#$%&'()*+0x2C:;<=>?@[\\]^`{0x7C~},startsnotwith=/,startsnotwith=-,startsnotwith=_,startsnotwith=.endsnotwith=/,endsnotwith=-,endsnotwith=_,endsnotwith=.")
	if errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			return "", errs
		}
		log.Fatalln(errs)
	}

	s := strings.Split(pkgName, "/")
	prjName := s[len(s)-1]

	errs = validate.Var(prjName, "max=100,startsnotwith=-,startsnotwith=_,startsnotwith=.,endsnotwith=-,endsnotwith=_,endsnotwith=.")
	if errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			return "", errs
		}
		log.Fatalln(errs)
	}

	return prjName, nil
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
