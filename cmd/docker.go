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
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

// dockerCmd represents the docker command
var dockerfileCmd = &cobra.Command{
	Use:   "dockerfile",
	Short: "Create a docker-compose.yml and Dockerfile to run your bean application locally",
	Long: `This command will create a Dockerfile and docker-compose.yml to help you run and test your application locally.
Example :- "bean create dockerfile".`,
	Args: cobra.ExactArgs(0),
	Run:  docker,
}

type Dockerfile struct {
	ProjectObject Project
	GoVersion     string
	AppPort       int
}

type DockerComposefile struct {
	ProjectObject Project
	AppPort       int
}

func init() {
	dockerfileCmd.DisableFlagsInUseLine = false
	createCmd.AddCommand(dockerfileCmd)
}

func docker(cmd *cobra.Command, args []string) {
	beanCheck := beanInitialisationCheck() // This function will display an error message on the terminal.
	if !beanCheck {
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dockerfilePath := wd + "/Dockerfile"
	// Check if Dockerfile already exists or not.
	_, err = os.Stat(dockerfilePath)
	if err == nil {
		fmt.Println("Dockerfile already exists!")
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

	// Set the PkgPath to replace the `{{.ProjectObject.PkgPath}}` placeholder in the Dockerfile.
	p.PkgPath, err = getPackagePathNameFromEnv(p)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Reading the base Dockerfile.
	file, err := p.RootFS.Open("Dockerfile")
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

	// Setting the go version in the Dockerfile
	version := runtime.Version()
	version = strings.TrimPrefix(version, "go")
	if version == "" {
		// Set a default version
		version = "1.20.5"
	}

	var dockerFile Dockerfile
	dockerFile.ProjectObject = *p
	dockerFile.GoVersion = strings.ToLower(version)
	dockerFile.AppPort = 8888

	dockerFileCreate, err := os.Create(dockerfilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer dockerFileCreate.Close()

	err = tmpl.Execute(dockerFileCreate, dockerFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Your Dockerfile has been created.")

	dockerComposefilePath := wd + "/docker-compose.yml"
	// Check if docker-compose.yml file already exists or not.
	_, err = os.Stat(dockerComposefilePath)
	if err == nil {
		fmt.Println("docker-compose.yml file already exists!")
		os.Exit(1)
	}

	// Reading the base docker-compose.yml.
	file, err = p.RootFS.Open("docker-compose.yml")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fileData, err = io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	tmpl, err = template.New("").Parse(string(fileData))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var dockerComposeFile DockerComposefile
	dockerComposeFile.ProjectObject = *p
	dockerComposeFile.AppPort = 8888

	dockerComposeFileCreate, err := os.Create(dockerComposefilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer dockerComposeFileCreate.Close()

	err = tmpl.Execute(dockerComposeFileCreate, dockerComposeFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Your docker-compose.yml file has been created.")
}
