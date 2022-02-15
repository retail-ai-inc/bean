// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	fpath "path"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// upgradeCmd represents the upgrade command
// It will replace the files inside the ./framework with the latest version files.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the framework files in the current project",
	Long: `Upgrade will update the latest framework filesin the current project. Error happens when the env.json
not found. Please run "go install github.com/retail-ai-inc/bean@latest" before using this command.`,
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {

		wd, err := os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}

		p := &Project{
			Copyright: `// MIT License

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
// SOFTWARE.`,
			RootDir: wd,
			SubDir:  "/framework",
		}

		// Set the relative root path of the internal FS.
		if p.RootFS, err = fs.Sub(InternalFS, "internal/project/framework"); err != nil {
			log.Fatalln(err)
		}

		// Read env.json to set proper project obejct.
		fmt.Println("loading env.json...")
		viper.SetConfigName("env")  // name of config file (without extension)
		viper.SetConfigType("json") // REQUIRED if the config file does not have the extension in the name
		viper.AddConfigPath(".")    // path to look for the config file in
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalln(err)
		}
		p.PkgPath = viper.GetString("packageName")
		p.PkgName = viper.GetString("projectName")

		fmt.Println("\nupdating framework files...")
		if err := fs.WalkDir(p.RootFS, ".", p.updateFrameworkFiles); err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func (p *Project) updateFrameworkFiles(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if d.IsDir() {
		// Create the same directory under current directory, skip if it already exists.
		if err := os.Mkdir(p.RootDir+p.SubDir+"/"+path, 0754); err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		// Create templates from the files.
		var temp *template.Template
		fmt.Println(p.RootDir + p.SubDir + "/" + path)

		if strings.HasSuffix(path, ".go") {
			// Preprocess bean directive for .go files.
			file, err := p.RootFS.Open(path)
			if err != nil {
				return err
			}

			content, err := ioutil.ReadAll(file)
			if err != nil {
				return err
			}

			result, err := PreprocessBeanDirective(string(content))
			if err != nil {
				return err
			}

			temp, err = template.New(path).Parse(result)
			if err != nil {
				return err
			}
		} else {
			// No preprocess for other files
			temp, err = template.ParseFS(p.RootFS, path)
			if err != nil {
				return err
			}
		}

		// For files which start with `.`, for example `.gitignore`.
		fileName := path
		fileNameWithoutPath := fpath.Base(path)
		if strings.HasPrefix(fileNameWithoutPath, "bean-dot") {
			parentPath := strings.TrimSuffix(path, fileNameWithoutPath)
			fileName = parentPath + strings.TrimPrefix(fileNameWithoutPath, "bean-dot")
		}

		file, err := os.Create(p.RootDir + p.SubDir + "/" + fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		// Parse the template and write to the files.
		fileTemplate := template.Must(temp, err)
		err = fileTemplate.Execute(file, p)
		if err != nil {
			return err
		}
	}

	return nil
}
