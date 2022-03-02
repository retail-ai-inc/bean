// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	serviceCmd.DisableFlagsInUseLine = true
	createCmd.AddCommand(serviceCmd)
}

type Service struct {
	ProjectObject    Project
	ServiceNameUpper string
	ServiceNameLower string
	RepoExists       bool
}

// serviceCmd represents the service command
var (
	serviceValidationRule = `A service name must satisfy the following requirements:-
	1. The serviceName should not start or end with slash (/).
	2. The serviceName should not start with any digit.
	3. The serviceName should not begin or end with a dot (., U+002E).
	4. The serviceName is a non-empty string made of up ASCII letters, ASCII digits, and limited ASCII punctuation (-, ., _).
	`
	serviceCmd = &cobra.Command{
		Use:   "service",
		Short: "Creates a new service",
		Long: `Command takes one argument that is the name of user-defined service
		Example :- "bean create service post" will create a service Post.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			beanCheck := beanInitialisationCheck()
			if !beanCheck {
				log.Fatalln("env.json for bean not found!!")
			}

			wd, err := os.Getwd()
			if err != nil {
				log.Fatalln(err)
			}

			userServiceName := args[0]
			serviceName, err := getServiceName(userServiceName)
			if err != nil {
				log.Fatalln(serviceValidationRule)
			}

			serviceFilesPath := wd + "/services/"
			serviceFileName := strings.ToLower(serviceName)

			// check if service already exists.
			_, err = os.Stat(serviceFilesPath + serviceFileName + ".go")
			if err == nil {
				log.Fatalln("Service with name " + serviceFileName + " already exists.")
			}

			p := &Project{
				Copyright:   copyright,
				RootDir:     wd,
				BeanVersion: rootCmd.Version,
			}

			// Set the relative root path of the internal templates folder.
			if p.RootFS, err = fs.Sub(InternalFS, "internal/tpl"); err != nil {
				log.Fatalln(err)
			}

			p.PkgPath, err = getPackagePathNameFromEnv(p)
			if err != nil {
				log.Fatalln(err)
				return
			}

			// Reading the base service file.
			baseServiceFilePath := "service.go"

			file, err := p.RootFS.Open(baseServiceFilePath)
			if err != nil {
				log.Fatalln(err)
				return
			}
			fileData, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatalln(err)
				return
			}

			tmpl, err := template.New("").Parse(string(fileData))
			if err != nil {
				log.Fatalln(err)
				return
			}

			var service Service
			// check if repo with same name exists then set template for service accordingly.
			repoCheck := checkRepoExists(serviceFileName)
			fmt.Println("repoExistsCheck", repoCheck)
			if repoCheck {
				service.RepoExists = true
			} else {
				service.RepoExists = false
			}
			service.ProjectObject = *p
			service.ServiceNameLower = strings.ToLower(serviceName)
			service.ServiceNameUpper = serviceName
			serviceFileCreate, err := os.Create(serviceFilesPath + serviceFileName + ".go")
			if err != nil {
				log.Println(err)
				return
			}
			defer serviceFileCreate.Close()

			err = tmpl.Execute(serviceFileCreate, service)
			if err != nil {
				log.Println(err)
				return
			}
			fmt.Printf("service with name %s and service file with name %s.go created\n", serviceName, serviceFileName)
		},
	}
)

func getServiceName(serviceName string) (string, error) {
	validate := validator.New()

	if unicode.IsDigit(rune(serviceName[0])) {
		fmt.Println("serviceName starts with digit")
		return "", errors.New("service name can not start with a digit")
	}

	if errs := validate.Var(serviceName, "required,max=100,startsnotwith=/,endsnotwith=/,alphanum,startsnotwith=.,endsnotwith=."); errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			fmt.Println(errs.Error())
			return "", errs
		}
		log.Fatalln(errs)
	}

	serviceName = strings.ToUpper(serviceName[:1]) + strings.ToLower(serviceName[1:])

	return serviceName, nil
}

func checkRepoExists(repoName string) bool {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	repoFilesPath := wd + "/repositories/"
	_, err = os.Stat(repoFilesPath + repoName + ".go")
	return err == nil
}

func getPackagePathNameFromEnv(p *Project) (string, error) {
	viper.AddConfigPath(p.RootDir)
	viper.SetConfigType("json")
	viper.SetConfigName("env")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln(err)
		return "", err
	}
	packagePath := viper.GetString("packagePath")
	// fmt.Println("packagePath", packagePath)
	return packagePath, nil
}
