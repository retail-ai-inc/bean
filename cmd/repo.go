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
	repoCmd.DisableFlagsInUseLine = true
	createCmd.AddCommand(repoCmd)
}

type Repo struct {
	ProjectObject Project
	RepoName      string
}

// repoCmd represents the repo command
var (
	repoValidationRule = `A repo name must satisfy the following requirements:-
	1. The repoName should not start or end with slash (/).
	2. The repoName should not start with any digit.
	3. The repoName should not begin or end with a dot (., U+002E).
	4. The repoName is a non-empty string made of up ASCII letters, ASCII digits, and limited ASCII punctuation (-, ., _).
	`

	repoCmd = &cobra.Command{
		Use:   "repo <repo-name>",
		Short: "Creates a new repository",
		Long: `Command takes one argument that is the name of user-defined repository
Example :- "bean create repo post" will create a repository Post in repositories folder.`,
		Args: cobra.ExactArgs(1),
		Run:  repo,
	}
)

func repo(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck()
	if !beanCheck {
		log.Fatalln("env.json for bean not found!!")
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	userRepoName := args[0]
	repoName, err := getRepoName(userRepoName)
	if err != nil {
		log.Fatalln(repoValidationRule)
	}

	repoFilesPath := wd + "/repositories/"
	repoFileName := strings.ToLower(repoName)

	// check if repo already exists.
	_, err = os.Stat(repoFilesPath + repoFileName + ".go")
	if err == nil {
		log.Fatalln("Repo with name " + repoFileName + " already exists.")
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

	// Reading the base repo file.
	baseRepoFilePath := "repo.go"

	file, err := p.RootFS.Open(baseRepoFilePath)
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

	var repo Repo
	repo.ProjectObject = *p
	repo.RepoName = repoName
	repoFileCreate, err := os.Create(repoFilesPath + repoFileName + ".go")
	if err != nil {
		log.Println(err)
		return
	}
	defer repoFileCreate.Close()

	err = tmpl.Execute(repoFileCreate, repo)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Printf("repository with name %s and repository file with name %s.go created\n", repoName, repoFileName)
}

func getRepoName(repoName string) (string, error) {
	validate := validator.New()

	if unicode.IsDigit(rune(repoName[0])) {
		fmt.Println("repoName starts with digit")
		return "", errors.New("repo name can not start with a digit")
	}

	if errs := validate.Var(repoName, "required,max=100,startsnotwith=/,endsnotwith=/,alphanum,startsnotwith=.,endsnotwith=."); errs != nil {
		if errs, ok := errs.(validator.ValidationErrors); ok {
			fmt.Println(errs.Error())
			return "", errs
		}
		log.Fatalln(errs)
	}

	repoName = strings.ToUpper(repoName[:1]) + strings.ToLower(repoName[1:])

	return repoName, nil
}
