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
	"github.com/spf13/viper"
)

var repoFlag []string

func init() {
	serviceCmd.DisableFlagsInUseLine = true

	serviceCmd.Flags().StringSliceVarP(&repoFlag, "repo", "r", []string{}, "Repository name i.e. --repo login --repo logout --repo profile,settings")

	createCmd.AddCommand(serviceCmd)
}

type Service struct {
	ProjectObject    Project
	ServiceNameUpper string
	ServiceNameLower string
	// Repos are the repositories injected into the service.
	Repos []ServiceRepo
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
		Use:   "service <service-name>",
		Short: "Create a new service file of your choice",
		Long: `Command takes one argument that is the name of user-defined service
Example :- "bean create service post" will create a service Post in the services folder.

Without --repo, the repository with the same name (repositories/post.go) is injected into the service if it exists.
With --repo, the given repositories are injected instead (and created if they do not exist):
  bean create service post --repo post,user`,
		Args: cobra.ExactArgs(1),
		Run:  service,
	}
)

func service(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck() // This function will display an error message on the terminal.
	if !beanCheck {
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	userServiceName := args[0]
	serviceName, err := getServiceName(userServiceName)
	if err != nil {
		fmt.Println(serviceValidationRule)
		os.Exit(1)
	}

	serviceFilesPath := wd + "/services/"
	serviceFileName := strings.ToLower(serviceName)

	// check if service already exists.
	_, err = os.Stat(serviceFilesPath + serviceFileName + ".go")
	if err == nil {
		fmt.Println("Service with name " + serviceFileName + " already exists.")
		os.Exit(1)
	}

	// The repositories injected into the service: the `--repo` ones (created if missing) or, without `--repo`,
	// the repository with the same name as the service if it exists.
	repos, err := serviceRepos(serviceFileName, repoFlag, checkRepoExists)
	if err != nil {
		fmt.Println(err)
		fmt.Println(repoValidationRule)
		os.Exit(1)
	}
	for _, r := range repos {
		if !checkRepoExists(r.Lower) {
			repo(cmd, []string{r.Lower})
		}
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

	// Reading the base service file.
	baseServiceFilePath := "service.go"

	file, err := p.RootFS.Open(baseServiceFilePath)
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

	service := Service{
		ProjectObject:    *p,
		ServiceNameUpper: serviceName,
		ServiceNameLower: strings.ToLower(serviceName),
		Repos:            repos,
	}

	serviceFileCreate, err := os.Create(serviceFilesPath + serviceFileName + ".go")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = tmpl.Execute(serviceFileCreate, service)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	_ = serviceFileCreate.Close()

	routerFilesPath := wd + "/routers/"
	lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "type Services struct {")
	if err == nil && lineNumber > 0 {
		textToInsert := `	` + service.ServiceNameLower + `Svc services.` + service.ServiceNameUpper + `Service` + ` // added by bean`
		_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)

		lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "svcs := &Services{")
		if err == nil && lineNumber > 0 {
			textToInsert := `		` + service.ServiceNameLower + `Svc: services.New` + service.ServiceNameUpper + `Service(` + serviceConstructorArgs(repos) + `),` + ` // added by bean`
			_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)
		}
	}

	fmt.Printf("service with name %s and service file %s.go created\n", serviceName, serviceFileName)
}

// ServiceRepo is a repository injected into a generated service.
type ServiceRepo struct {
	Upper string // e.g. `Post`, as in `repositories.PostRepository`
	Lower string // e.g. `post`, as in `repos.postRepo` and the file `repositories/post.go`
}

// serviceRepos returns the repositories to inject into the service: the `--repo` ones (normalized like
// `bean create repo` does, duplicates removed) or, without `--repo`, the repository with the same name as the
// service if it exists.
func serviceRepos(serviceFileName string, repoNames []string, repoExists func(string) bool) ([]ServiceRepo, error) {
	if len(repoNames) == 0 {
		if repoExists(serviceFileName) {
			return []ServiceRepo{{Upper: strings.ToUpper(serviceFileName[:1]) + serviceFileName[1:], Lower: serviceFileName}}, nil
		}
		return nil, nil
	}

	var repos []ServiceRepo
	seen := map[string]bool{}
	for _, name := range repoNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		repoName, err := getRepoName(name)
		if err != nil {
			return nil, fmt.Errorf("invalid repository name %q", name)
		}
		lower := strings.ToLower(repoName)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		repos = append(repos, ServiceRepo{Upper: repoName, Lower: lower})
	}
	return repos, nil
}

// serviceConstructorArgs returns the arguments of the service constructor call in `routers/route.go`.
func serviceConstructorArgs(repos []ServiceRepo) string {
	args := make([]string, 0, len(repos))
	for _, r := range repos {
		args = append(args, "repos."+r.Lower+"Repo")
	}
	return strings.Join(args, ", ")
}

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
		fmt.Println(errs)
		os.Exit(1)
	}

	serviceName = strings.ToUpper(serviceName[:1]) + strings.ToLower(serviceName[1:])

	return serviceName, nil
}

func checkRepoExists(repoName string) bool {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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
		return "", err
	}
	packagePath := viper.GetString("packagePath")
	return packagePath, nil
}
