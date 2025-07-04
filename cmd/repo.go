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
	RepoNameUpper string
	RepoNameLower string
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
		Short: "Create a new repository file of your choice",
		Long: `Command takes one argument that is the name of user-defined repository
Example :- "bean create repo post" will create a repository Post in repositories folder.`,
		Args: cobra.ExactArgs(1),
		Run:  repo,
	}
)

func repo(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck() // This function will display an error message on the terminal.
	if !beanCheck {
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	userRepoName := args[0]
	repoName, err := getRepoName(userRepoName)
	if err != nil {
		fmt.Println(repoValidationRule)
		os.Exit(1)
	}

	repoFilesPath := wd + "/repositories/"
	repoFileName := strings.ToLower(repoName)

	// check if repo already exists.
	_, err = os.Stat(repoFilesPath + repoFileName + ".go")
	if err == nil {
		fmt.Println("Repo with name " + repoFileName + " already exists.")
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

	// Reading the base repo file.
	baseRepoFilePath := "repo.go"

	file, err := p.RootFS.Open(baseRepoFilePath)
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

	var repo Repo
	repo.ProjectObject = *p
	repo.RepoNameLower = strings.ToLower(repoName)
	repo.RepoNameUpper = repoName

	repoFileCreate, err := os.Create(repoFilesPath + repoFileName + ".go")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func(file2 *os.File) {
		_ = file2.Close()
	}(repoFileCreate)

	err = tmpl.Execute(repoFileCreate, repo)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	routerFilesPath := wd + "/routers/"
	lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "type Repositories struct {")
	if err == nil && lineNumber > 0 {
		textToInsert := `	` + repo.RepoNameLower + `Repo repositories.` + repo.RepoNameUpper + `Repository` + ` // added by bean`
		_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)

		lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "repos := &Repositories{")
		if err == nil && lineNumber > 0 {
			textToInsert := `		` + repo.RepoNameLower + `Repo: repositories.New` + repo.RepoNameUpper + `Repository(b.DBConn),` + ` // added by bean`
			_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)
		}
	}

	fmt.Printf("repository with name %s and repository file %s.go created\n", repoName, repoFileName)
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
		fmt.Println(errs)
		os.Exit(1)
	}

	repoName = strings.ToUpper(repoName[:1]) + strings.ToLower(repoName[1:])

	return repoName, nil
}
