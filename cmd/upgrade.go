// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// upgradeCmd represents the upgrade command
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
			Copyright: `// Copyright The RAI Inc.
// The RAI Authors`,
			RootDir: wd,
			SubDir:  "/framework",
		}

		// Set the relative root path of the internal FS.
		if p.RootFS, err = fs.Sub(InternalFS, "internal/project/framework"); err != nil {
			log.Fatalln(err)
		}

		// Read env.json to set proper project obejct.
		fmt.Println("\nloading env.json...")
		viper.SetConfigName("env")  // name of config file (without extension)
		viper.SetConfigType("json") // REQUIRED if the config file does not have the extension in the name
		viper.AddConfigPath(".")    // path to look for the config file in
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalln(err)
		}
		p.PrjName = viper.GetString("projectName")
		p.PkgName = viper.GetString("packageName")

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

	fmt.Println(path)

	if d.IsDir() {
		// Create the same directory under current directory, skip if it already exists.
		if err := os.Mkdir(p.RootDir+p.SubDir+"/"+path, 0754); err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		// Overwrite exisiting framework files.
		fileName := strings.TrimSuffix(path, ".tpl")
		fmt.Println(fileName)
		file, err := os.Create(p.RootDir + p.SubDir + "/" + fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		// Add executable permission to shell script.
		if fileName == "bean.sh" {
			if err := file.Chmod(0755); err != nil {
				return err
			}
		}

		// Parse the template and write to the files.
		fileTemplate := template.Must(template.ParseFS(p.RootFS, path))
		err = fileTemplate.Execute(file, p)
		if err != nil {
			return err
		}
	}

	return nil
}
