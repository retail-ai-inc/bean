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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	serviceCmd.DisableFlagsInUseLine = true

	serviceCmd.Flags().StringSliceP("repo", "r", []string{}, "Repository name i.e. --repo login --repo logout --repo profile,settings")

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
var serviceCmd = &cobra.Command{
	Use:   "service <service-name>",
	Short: "Create a new service file of your choice",
	Long: `Command takes one argument that is the name of user-defined service
(1 to 100 ASCII letters or digits, not starting with a digit).
Example :- "bean create service post" will create a service Post in the services folder.

Without --repo, the repository with the same name (repositories/post.go) is injected into the service if it exists.
With --repo, the given repositories are injected instead (and created if they do not exist):
  bean create service post --repo post,user`,
	Args: cobra.ExactArgs(1),
	RunE: service,
}

func service(cmd *cobra.Command, args []string) error {
	p, err := loadProject()
	if err != nil {
		return err
	}

	name, err := getServiceName(args[0])
	if err != nil {
		return err
	}
	lower := strings.ToLower(name)
	if fileExists(p.RootDir, "services", lower+".go") {
		return fmt.Errorf("services/%s.go already exists", lower)
	}

	// The repositories injected into the service: the `--repo` ones (created if missing) or, without `--repo`,
	// the repository with the same name as the service if it exists.
	repoFlag, err := cmd.Flags().GetStringSlice("repo")
	if err != nil {
		return err
	}
	repos, err := serviceRepos(lower, repoFlag, func(name string) bool {
		return fileExists(p.RootDir, "repositories", name+".go")
	})
	if err != nil {
		return err
	}
	for _, r := range repos {
		if !fileExists(p.RootDir, "repositories", r.Lower+".go") {
			if err := createRepo(p, r.Upper); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "repository with name %s and repository file %s.go created\n", r.Upper, r.Lower)
		}
	}

	data := Service{ProjectObject: *p, ServiceNameUpper: name, ServiceNameLower: lower, Repos: repos}
	if err := generateFile(p, "service.go", filepath.Join("services", lower+".go"), data); err != nil {
		return err
	}
	if err := wireRoute(p.RootDir, "Services",
		lower+"Svc services."+name+"Service",
		lower+"Svc: services.New"+name+"Service("+serviceConstructorArgs(repos)+")",
	); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "service with name %s and service file %s.go created\n", name, lower)
	return nil
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
			return nil, err
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

// getServiceName validates a service name and returns it like `Post`.
func getServiceName(serviceName string) (string, error) {
	return componentName("service", serviceName)
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
