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
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// dockerCmd represents the docker command
var dockerfileCmd = &cobra.Command{
	Use:   "dockerfile",
	Short: "Create a Dockerfile, docker-compose.yml and .air.toml to run your bean application locally",
	Long: `This command will create a Dockerfile and docker-compose.yml to help you run and test your application locally.
Example :- "bean create dockerfile".`,
	Args: cobra.ExactArgs(0),
	Run:  docker,
}

type Dockerfile struct {
	ProjectObject Project
	GoVersion     string
	AppPort       string
}

type DockerComposefile struct {
	ProjectObject Project
	ServiceName   string // service, container and host name
	AppPort       string
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

	appPort := viper.GetString("http.port") // env.json was read by getPackagePathNameFromEnv
	if appPort == "" {
		appPort = config.DefaultHTTPPort
	}

	files := []struct {
		tpl, path string
		data      any
	}{
		{"Dockerfile", dockerfilePath, Dockerfile{ProjectObject: *p, GoVersion: projectGoVersion(wd), AppPort: appPort}},
		{"docker-compose.yml", filepath.Join(wd, "docker-compose.yml"), DockerComposefile{ProjectObject: *p, ServiceName: dockerName(p.PkgPath), AppPort: appPort}},
		{"air.toml", filepath.Join(wd, ".air.toml"), nil},
	}
	for _, f := range files {
		if _, err := os.Stat(f.path); err == nil {
			fmt.Printf("%s already exists, not overwritten.\n", filepath.Base(f.path))
			continue
		}
		if err := renderTemplateFile(p.RootFS, f.tpl, f.path, f.data); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("Your %s has been created.\n", filepath.Base(f.path))
	}
}

// renderTemplateFile renders the template tpl of fsys to the file path.
func renderTemplateFile(fsys fs.FS, tpl, path string, data any) error {
	content, err := fs.ReadFile(fsys, tpl)
	if err != nil {
		return err
	}
	tmpl, err := template.New(tpl).Parse(string(content))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// projectGoVersion returns the `major.minor` Go version of the project's go.mod (the golang image tag), or of the
// running Go if go.mod cannot be read.
func projectGoVersion(dir string) string {
	version := ""
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
				version = strings.TrimSpace(v)
				break
			}
		}
	}
	if version == "" {
		version = strings.TrimPrefix(runtime.Version(), "go")
	}
	// Keep `major.minor`: patch tags of the image lag behind, and a development toolchain has no image.
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 || !isDigits(parts[0]) || !isDigits(parts[1]) {
		return "1"
	}
	return parts[0] + "." + parts[1]
}

func isDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}

// dockerName returns a valid container / host name for the module path: its last element, lower case, with any
// character other than [a-z0-9-] replaced by "-".
func dockerName(pkgPath string) string {
	name := strings.ToLower(pkgPath[strings.LastIndex(pkgPath, "/")+1:])
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-")
	if name == "" {
		return "app"
	}
	return name
}
