// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"io/fs"
	"log"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// InternalFS is a global variable shared within the whole cmd package. It is set in
// the `Execute()` when main function run because go embed cannot access parent directory.
var InternalFS fs.FS

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
func Execute(internalFS fs.FS) {
	InternalFS = internalFS
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		log.Fatalln("Failed to read build info")
	}

	rootCmd.Version = bi.Main.Version

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
