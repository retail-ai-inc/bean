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
)

func init() {
	handlerCmd.DisableFlagsInUseLine = true
	createCmd.AddCommand(handlerCmd)
}

type Handler struct {
	ProjectObject    Project
	HandlerNameUpper string
	HandlerNameLower string
	ServiceExists    bool
}

// handlerCmd represents the handler command
var (
	handlerValidationRule = `A handler name must satisfy the following requirements:-
	1. The handlereName should not start or end with slash (/).
	2. The handlereName should not start with any digit.
	3. The handlereName should not begin or end with a dot (., U+002E).
	4. The handlereName is a non-empty string made of up ASCII letters, ASCII digits, and limited ASCII punctuation (-, ., _).
	`

	handlerCmd = &cobra.Command{
		Use:   "handler",
		Short: "Creates a handler",
		Long: `Command takes one argument that is the name of user-defined handler
		Example :- "bean create handler post" will create a handler Post.`,
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

			userHandlerName := args[0]
			handlerName, err := getHandlerName(userHandlerName)
			// fmt.Println("handlerName", handlerName)
			if err != nil {
				log.Fatalln(handlerValidationRule)
			}

			handlerFilesPath := wd + "/handlers/"
			handlerFileName := strings.ToLower(handlerName)

			// check if handler already exists.
			_, err = os.Stat(handlerFilesPath + handlerFileName + ".go")
			if err == nil {
				log.Fatalln("Handler with name " + handlerFileName + " already exists.")
			}

			p := &Project{
				Copyright:   copyright,
				RootDir:     wd,
				BeanVersion: rootCmd.Version,
			}

			// Set the relative root path of the internal templates folder.
			if p.RootFS, err = fs.Sub(InternalFS, "internal/_tpl"); err != nil {
				log.Fatalln(err)
			}
			// fmt.Println("InternalFS", InternalFS)
			// fmt.Println("p.RootFS", p.RootFS)
			// fmt.Println("p.RootDir", p.RootDir)

			p.PkgPath, err = getPackagePathNameFromEnv(p)
			if err != nil {
				log.Fatalln(err)
				return
			}

			// Reading the base handler file.
			baseHandlerFilePath := "handler.go"

			file, err := p.RootFS.Open(baseHandlerFilePath)
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
			// fmt.Println("HandlerFile Path!!" + handlerFilesPath + handlerFileName + ".go")

			var handler Handler
			// check if service with same name exists then set template for handler accordingly.
			serviceCheck := checkServiceExists(handlerFileName)
			fmt.Println("serviceExistsCheck", serviceCheck)
			if serviceCheck {
				handler.ServiceExists = true
			} else {
				handler.ServiceExists = false
			}
			handler.ProjectObject = *p
			handler.HandlerNameLower = strings.ToLower(handlerName)
			handler.HandlerNameUpper = handlerName
			handlerFileCreate, err := os.Create(handlerFilesPath + handlerFileName + ".go")
			if err != nil {
				log.Println(err)
				return
			}
			defer handlerFileCreate.Close()

			err = tmpl.Execute(handlerFileCreate, handler)
			if err != nil {
				log.Println(err)
				return
			}
			fmt.Printf("handler with name %s and handler file with name %s.go created\n", handlerName, handlerFileName)
		},
	}
)

func getHandlerName(handlerName string) (string, error) {
	validate := validator.New()

	if unicode.IsDigit(rune(handlerName[0])) {
		fmt.Println("handlerName starts with digit")
		return "", errors.New("handler name can not start with a digit")
	}

	if errs := validate.Var(handlerName, "required,max=100,startsnotwith=/,endsnotwith=/,alphanum,startsnotwith=.,endsnotwith=."); errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			fmt.Println(errs.Error())
			return "", errs
		}
		log.Fatalln(errs)
	}

	handlerName = strings.ToUpper(handlerName[:1]) + strings.ToLower(handlerName[1:])

	return handlerName, nil
}

func checkServiceExists(repoName string) bool {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	serviceFilesPath := wd + "/services/"
	_, err = os.Stat(serviceFilesPath + repoName + ".go")
	return err == nil
}
