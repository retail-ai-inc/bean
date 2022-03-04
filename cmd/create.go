// Copyright The RAI Inc.
// The RAI Authors
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	createCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(createCmd)
}

var createCmd = &cobra.Command{
	Use:       "create",
	Short:     "Enables user to create new base command, repository, service and handler template.",
	Long:      `This command requires a sub command parameter to create a new command, repository, service and handler template.`,
	Args:      cobra.ExactValidArgs(1),
	ValidArgs: []string{"repo", "service", "handler", "command"},
}

func beanInitialisationCheck() bool {
	if _, err := os.Stat("env.json"); errors.Is(err, os.ErrNotExist) {
		fmt.Println("env.json not found")
		return false
	}
	return true
}
