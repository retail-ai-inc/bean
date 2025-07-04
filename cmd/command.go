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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strings"
	"text/template"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
)

func init() {
	createCmd.AddCommand(commandCmd)
}

type Command struct {
	ProjectObject Project
	CommandName   string
}

// commandCmd represents the command command
var (
	commandValidationRule = `A command-name must satisfy the following requirements:-
	1. The command-name should not begin or end with slash (/).
	2. The command-name should not begin with any digit.
	3. The command-name should not begin or end with a dot (., U+002E).
	4. The command-name is a non-empty string made of up ASCII letters, ASCII digits, and limited ASCII punctuation (-, ., _).
	5. The command-name cannot contain more than 100 characters.
	`
	commandCmd = &cobra.Command{
		Use:   "command <command-name>",
		Short: "Create a new command file of your choice",
		Long: `Command takes one argument that is the name of user-defined command
Example :- "bean create command test" will create a command test in the commands folder.`,
		Args: cobra.ExactArgs(1),
		Run:  command,
	}
)

func command(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck() // This function will display an error message on the terminal.
	if !beanCheck {
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	userCommandName := args[0]
	commandName, err := getCommandName(userCommandName)
	if err != nil {
		fmt.Println(commandValidationRule)
		os.Exit(1)
	}

	commandFilesPath := wd + "/commands/"
	commandFileName := strings.ToLower(commandName)

	// check if command already exists.
	_, err = os.Stat(commandFilesPath + commandFileName + ".go")
	if err == nil {
		fmt.Println("Command with name " + commandFileName + " already exists.")
		return
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

	// Reading the base command file.
	baseCommandFilePath := "command.go"

	file, err := p.RootFS.Open(baseCommandFilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fileData, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	tmpl, err := template.New("").Parse(string(fileData))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var command Command
	command.ProjectObject = *p
	command.CommandName = commandName
	commandFileCreate, err := os.Create(commandFilesPath + commandFileName + ".go")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func(f *os.File) {
		if err := f.Close(); err != nil {
			log.Printf("Failed to close file:%v", err)
		}
	}(commandFileCreate)

	err = tmpl.Execute(commandFileCreate, command)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("command with name %s and command file %s.go created\n", commandName, commandFileName)
}

func getCommandName(commandName string) (string, error) {
	validate := validator.New()

	if unicode.IsDigit(rune(commandName[0])) {
		fmt.Println("commandName starts with digit")
		return "", errors.New("command name can not start with a digit")
	}

	if errs := validate.Var(commandName, "required,max=100,startsnotwith=/,endsnotwith=/,alphanum,startsnotwith=.,endsnotwith=."); errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			return "", errs
		}
	}

	commandName = strings.ToLower(commandName)

	return commandName, nil
}
