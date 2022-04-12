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
		Use:   "service <service-name>",
		Short: "Create a new service file of your choice",
		Long: `Command takes one argument that is the name of user-defined service
Example :- "bean create service post" will create a service Post in the services folder.`,
		Args: cobra.ExactArgs(1),
		Run:  service,
	}
)

func service(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck()
	if !beanCheck {
		log.Fatalln("env.json for bean not found!!")
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	var reposParamForSvcs []string

	if len(repoFlag) > 0 {
		for _, v := range repoFlag {
			repoFile := wd + "/repositories/" + v + ".go"

			if _, err := os.Stat(repoFile); errors.Is(err, os.ErrNotExist) {
				// repository file does not exist so, create it
				repo(cmd, []string{v})
			}

			reposParamForSvcs = append(reposParamForSvcs, "repos."+strings.ToLower(v)+"Repo")
		}
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
	if p.RootFS, err = fs.Sub(InternalFS, "internal/_tpl"); err != nil {
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

	err = tmpl.Execute(serviceFileCreate, service)
	if err != nil {
		log.Println(err)
		return
	}

	// IMPORTANT: Instead of `defer` let's close the service file so that we can open it again if requires.
	serviceFileCreate.Close()

	if len(repoFlag) > 1 {
		// Open service file and update 3 places
		serviceFileToUpdate := serviceFilesPath + serviceFileName + ".go"
		needle := service.ServiceNameLower + "Repository repositories." + service.ServiceNameUpper + "Repository"
		lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(serviceFileToUpdate, needle)

		if err == nil && lineNumber > 0 {
			newText := ""

			for _, v := range repoFlag {
				newText += "\t" + strings.ToLower(v) + "Repository repositories." +
					strings.ToUpper(v[:1]) + strings.ToLower(v[1:]) + "Repository\n"
			}

			replaceStringToNthLineOfFile(serviceFileToUpdate, newText, lineNumber)

			needle := "func New" + service.ServiceNameUpper + "Service("
			lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(serviceFileToUpdate, needle)

			if err == nil && lineNumber > 0 {
				var param []string

				for _, v := range repoFlag {
					param = append(param, strings.ToLower(v)+"Repo repositories."+
						strings.ToUpper(v[:1])+strings.ToLower(v[1:])+"Repository")
				}

				newText := "func New" + service.ServiceNameUpper + "Service(" +
					strings.Join(param[:], ", ") + ") *" + service.ServiceNameLower + "Service {"

				replaceStringToNthLineOfFile(serviceFileToUpdate, newText, lineNumber)

				needle := "return &" + service.ServiceNameLower + "Service{"
				lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(serviceFileToUpdate, needle)

				if err == nil && lineNumber > 0 {
					var param []string
					for _, v := range repoFlag {
						param = append(param, "\t\t"+strings.ToLower(v)+"Repository: "+strings.ToLower(v)+"Repo,")
					}

					// Remove `return &` and rewrite it again with new code
					err = replaceStringFromFileByRegex(
						serviceFileToUpdate, `return &`+service.ServiceNameLower+`Service{([^"]*)}`,
						"return &"+service.ServiceNameLower+"Service{",
						"",
					)
					if err == nil {
						newText := "\treturn &" + service.ServiceNameLower + "Service{\n" +
							strings.Join(param[:], "\n") + "\n\t}\n}"

						replaceStringToNthLineOfFile(serviceFileToUpdate, newText, lineNumber)
					}
				}
			}
		} else {

			needle := "type " + service.ServiceNameLower + "Service struct {}"
			lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(serviceFileToUpdate, needle)

			if err == nil {
				err = replaceStringFromFileByExactMatches(serviceFileToUpdate, needle, "")
				if err == nil {
					newText := "type " + service.ServiceNameLower + "Service struct {\n"

					for _, v := range repoFlag {
						newText += "\t" + strings.ToLower(v) + "Repository repositories." +
							strings.ToUpper(v[:1]) + strings.ToLower(v[1:]) + "Repository\n"
					}

					newText += "}"

					replaceStringToNthLineOfFile(serviceFileToUpdate, newText, lineNumber)

					needle = "func New" + service.ServiceNameUpper + "Service() *" + service.ServiceNameLower + "Service {"
					lineNumber, err = matchTextInFileAndReturnFirstOccurrenceLineNumber(serviceFileToUpdate, needle)

					if err == nil {
						err = replaceStringFromFileByExactMatches(serviceFileToUpdate, needle, "")

						if err == nil {
							var param []string

							for _, v := range repoFlag {
								param = append(param, strings.ToLower(v)+"Repo repositories."+
									strings.ToUpper(v[:1])+strings.ToLower(v[1:])+"Repository")
							}

							newText := "func New" + service.ServiceNameUpper + "Service(" +
								strings.Join(param[:], ", ") + ") *" + service.ServiceNameLower + "Service {"

							err = replaceStringToNthLineOfFile(serviceFileToUpdate, newText, lineNumber)
							if err == nil {
								needle = "return &" + service.ServiceNameLower + "Service{}"
								lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(serviceFileToUpdate, needle)
								if err == nil {
									err = replaceStringFromFileByExactMatches(serviceFileToUpdate, needle, "")
									if err == nil {
										var param []string
										for _, v := range repoFlag {
											param = append(param, "\t\t"+strings.ToLower(v)+"Repository: "+strings.ToLower(v)+"Repo,\n")
										}

										newText := "\treturn &" + service.ServiceNameLower + "Service{\n" + strings.Join(param[:], "") + "\t}"
										replaceStringToNthLineOfFile(serviceFileToUpdate, newText, lineNumber)

										needle = `// "github.com/retail-ai-inc/bean/helpers"`
										lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(serviceFileToUpdate, needle)
										if err == nil {
											replaceStringToNthLineOfFile(serviceFileToUpdate, "\t\""+p.PkgPath+`/repositories"`, lineNumber+1)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	routerFilesPath := wd + "/routers/"
	lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "type Services struct {")
	if err == nil && lineNumber > 0 {
		textToInsert := `	` + service.ServiceNameLower + `Svc services.` + service.ServiceNameUpper + `Service` + ` // added by bean`
		_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)

		lineNumber, err := matchTextInFileAndReturnFirstOccurrenceLineNumber(routerFilesPath+"route.go", "svcs := &Services{")
		if err == nil && lineNumber > 0 {
			textToInsert := `		` + service.ServiceNameLower + `Svc: services.New` + service.ServiceNameUpper + `Service(` + strings.Join(reposParamForSvcs[:], ", ") + `),` + ` // added by bean`
			_ = insertStringToNthLineOfFile(routerFilesPath+"route.go", textToInsert, lineNumber+1)
		}
	}

	fmt.Printf("service with name %s and service file %s.go created\n", serviceName, serviceFileName)
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
	return packagePath, nil
}
