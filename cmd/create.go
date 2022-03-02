// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	createCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(createCmd)
}

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:       "create",
	Short:     "Enables user to create new base command, repository, service and handler template.",
	Long:      `This command requires a sub command parameter to create a new command, repository, service and handler template.`,
	Args:      cobra.ExactValidArgs(1),
	ValidArgs: []string{"repo", "service", "handler", "command"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("create command called")
	},
}
