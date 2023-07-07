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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"regexp"
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
		Use:   "handler <handler-name>",
		Short: "Create a new handler file of your choice",
		Long: `Command takes one argument that is the name of user-defined handler
Example :- "bean create handler post" will create a handler Post in handlers folder.`,
		Args: cobra.ExactArgs(1),
		Run:  handler}
)

func handler(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck() // This function will display an error message on the terminal.
	if !beanCheck {
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	userHandlerName := args[0]
	handlerName, err := getHandlerName(userHandlerName)
	if err != nil {
		fmt.Println(handlerValidationRule)
		os.Exit(1)
	}

	handlerFilesPath := wd + "/handlers/"
	handlerFileName := strings.ToLower(handlerName)

	// check if handler already exists.
	_, err = os.Stat(handlerFilesPath + handlerFileName + ".go")
	if err == nil {
		fmt.Println("Handler with name " + handlerFileName + " already exists.")
		os.Exit(1)
	}

	p := &Project{
		Copyright: copyright,
		RootDir:   wd,
	}

	// Set the relative root path of the internal templates folder.
	if p.RootFS, err = fs.Sub(InternalFS, "internal/_tpl"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	p.PkgPath, err = getPackagePathNameFromEnv(p)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Reading the base handler file.
	baseHandlerFilePath := "handler.go"

	file, err := p.RootFS.Open(baseHandlerFilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer file.Close()

	fileData, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	tmpl, err := template.New("").Parse(string(fileData))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var handler Handler
	// check if service with same name exists then set template for handler accordingly.
	serviceCheck := checkServiceExists(handlerFileName)
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
		fmt.Println(err)
		os.Exit(1)
	}

	defer handlerFileCreate.Close()

	err = tmpl.Execute(handlerFileCreate, handler)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	routerFilesPath := wd + "/routers/"
	lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "type Handlers struct {")
	if err == nil && lineNumber > 0 {
		textToInsert := `	` + handler.HandlerNameLower + `Hdlr handlers.` + handler.HandlerNameUpper + `Handler` + ` // added by bean`
		_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)

		lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "hdlrs := &Handlers{")
		if err == nil && lineNumber > 0 {
			var textToInsert string

			if handler.ServiceExists {
				textToInsert = `		` + handler.HandlerNameLower + `Hdlr: handlers.New` + handler.HandlerNameUpper + `Handler(svcs.` + handler.HandlerNameLower + `Svc),` + ` // added by bean`
			} else {
				textToInsert = `		` + handler.HandlerNameLower + `Hdlr: handlers.New` + handler.HandlerNameUpper + `Handler(),` + ` // added by bean`
			}

			_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)
		}
	}

	fmt.Printf("handler with name %s and handler file %s.go created\n", handlerName, handlerFileName)
}

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
		fmt.Println(errs)
		os.Exit(1)
	}

	handlerName = strings.ToUpper(handlerName[:1]) + strings.ToLower(handlerName[1:])

	return handlerName, nil
}

func checkServiceExists(repoName string) bool {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	serviceFilesPath := wd + "/services/"
	_, err = os.Stat(serviceFilesPath + repoName + ".go")

	return err == nil
}

func matchTextInFileAndReturnFirstOccurrenceLineNumber(filePath string, needle string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	line := 1

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), needle) {
			return line, nil
		}

		line++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return 0, nil
}

// Insert sting to n-th line of file.
func insertStringToNthLineOfFile(filePath, textToInsert string, lineNumber int) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	fileContent := ""
	for i, line := range lines {
		if i == lineNumber {
			fileContent += textToInsert + "\n"
		}
		fileContent += line
		fileContent += "\n"
	}

	return ioutil.WriteFile(filePath, []byte(fileContent), 0644)
}

// Replace sting to n-th line of file.
func replaceStringToNthLineOfFile(filePath, newText string, lineNumber int) error {

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	fileContent := ""
	for i, line := range lines {
		if i == lineNumber-1 {
			fileContent += newText + "\n"
		} else {
			fileContent += line + "\n"
		}
	}

	return ioutil.WriteFile(filePath, []byte(fileContent), 0644)
}

func replaceStringFromFileByRegex(filePath, regex, additonal, replaceWith string) error {

	input, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(regex)
	match := re.FindStringSubmatch(string(input))
	if len(match) > 1 {
		replaceMe := additonal + match[1]
		output := bytes.Replace(input, []byte(replaceMe), []byte(replaceWith), -1)

		if err = ioutil.WriteFile(filePath, output, 0664); err != nil {
			return err
		}
	}

	return nil
}

func replaceStringFromFileByExactMatches(filePath, replaceMe, replaceWith string) error {
	input, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	output := bytes.Replace(input, []byte(replaceMe), []byte(replaceWith), -1)
	if err = ioutil.WriteFile(filePath, output, 0664); err != nil {
		return err
	}

	return nil
}
