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
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:       "create",
	Short:     "Enable user to create new handler, service, repository, command and docker file",
	Long:      `This command requires a sub command parameter to create a new command, repository, service, handler and docker template.`,
	Args:      cobra.ExactValidArgs(1),
	ValidArgs: []string{"repo", "service", "handler", "command", "docker"},
}

func init() {
	createCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(createCmd)
}

func beanInitialisationCheck() bool {
	if _, err := os.Stat("env.json"); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Your application env.json is not found!")
		return false
	}
	return true
}
