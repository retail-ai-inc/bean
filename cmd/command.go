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
	createCmd.AddCommand(commandCmd)
}

type Command struct {
	ProjectObject Project
	CommandName   string
}

// commandCmd represents the command command
var (
	commandValidationRule = `A command name must satisfy the following requirements:-
	1. The commandName should not start or end with slash (/).
	2. The commandName should not start with any digit.
	3. The commandName should not begin or end with a dot (., U+002E).
	4. The commandName is a non-empty string made of up ASCII letters, ASCII digits, and limited ASCII punctuation (-, ., _).
	`
	commandCmd = &cobra.Command{
		Use:   "command <command-name>",
		Short: "Creates a new command",
		Long: `Command takes one argument that is the name of user-defined command
Example :- "bean create command test" will create a command test in the commands folder.`,
		Args: cobra.ExactArgs(1),
		Run:  command,
	}
)

func command(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck()
	if !beanCheck {
		log.Fatalln("env.json for bean not found!!")
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	userCommandName := args[0]
	commandName, err := getCommandName(userCommandName)
	if err != nil {
		log.Fatalln(commandValidationRule)
	}

	commandFilesPath := wd + "/commands/"
	commandFileName := strings.ToLower(commandName)

	// check if command already exists.
	_, err = os.Stat(commandFilesPath + commandFileName + ".go")
	if err == nil {
		log.Fatalln("Command with name " + commandFileName + " already exists.")
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

	// Reading the base command file.
	baseCommandFilePath := "command.go"

	file, err := p.RootFS.Open(baseCommandFilePath)
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

	var command Command
	command.ProjectObject = *p
	command.CommandName = commandName
	commandFileCreate, err := os.Create(commandFilesPath + commandFileName + ".go")
	if err != nil {
		log.Println(err)
		return
	}
	defer commandFileCreate.Close()

	err = tmpl.Execute(commandFileCreate, command)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Printf("command with name %s and command file with name %s.go created\n", commandName, commandFileName)
}

func getCommandName(commandName string) (string, error) {
	validate := validator.New()

	if unicode.IsDigit(rune(commandName[0])) {
		fmt.Println("commandName starts with digit")
		return "", errors.New("command name can not start with a digit")
	}

	if errs := validate.Var(commandName, "required,max=100,startsnotwith=/,endsnotwith=/,alphanum,startsnotwith=.,endsnotwith=."); errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			fmt.Println(errs.Error())
			return "", errs
		}
		log.Fatalln(errs)
	}

	commandName = strings.ToLower(commandName)

	return commandName, nil
}
