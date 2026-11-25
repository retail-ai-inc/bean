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
var repoCmd = &cobra.Command{
	Use:   "repo <repo-name>",
	Short: "Create a new repository file of your choice",
	Long: `Command takes one argument that is the name of user-defined repository
(1 to 100 ASCII letters or digits, not starting with a digit).
Example :- "bean create repo post" will create a repository Post in repositories folder.`,
	Args: cobra.ExactArgs(1),
	RunE: repo,
}

func repo(cmd *cobra.Command, args []string) error {
	p, err := loadProject()
	if err != nil {
		return err
	}

	name, err := getRepoName(args[0])
	if err != nil {
		return err
	}
	if err := createRepo(p, name); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "repository with name %s and repository file %s.go created\n", name, strings.ToLower(name))
	return nil
}

// createRepo generates the repository `repositories/<name>.go` and wires it into routers/route.go.
func createRepo(p *Project, name string) error {
	lower := strings.ToLower(name)
	data := Repo{ProjectObject: *p, RepoNameUpper: name, RepoNameLower: lower}
	if err := generateFile(p, "repo.go", filepath.Join("repositories", lower+".go"), data); err != nil {
		return err
	}
	return wireRoute(p.RootDir, "Repositories",
		lower+"Repo repositories."+name+"Repository",
		lower+"Repo: repositories.New"+name+"Repository(b.DBConn)",
	)
}

// getRepoName validates a repository name and returns it like `Post`.
func getRepoName(repoName string) (string, error) {
	return componentName("repository", repoName)
}
