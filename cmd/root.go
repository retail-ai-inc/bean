// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"go/build"
	"log"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	ModulePath string
	Version    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bean command [args...]",
	Short: "A web framework written in GO on-top of echo to ease your development.",
	Long: `Bean is a web framework written in GO on-top of echo to ease your development.
This CLI application is a tool to generate the needed files to quickly create
a bean structured application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		log.Fatalln("Failed to read build info")
	}

	Version = bi.Main.Version
	ModulePath = gopath + "/pkg/mod/" + bi.Main.Path + "@" + bi.Main.Version

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.Version = Version
}
