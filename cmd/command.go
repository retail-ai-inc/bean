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
	"go/token"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	createCmd.AddCommand(commandCmd)
}

type Command struct {
	ProjectObject Project
	CommandName   string
}

// commandCmd represents the command command
var commandCmd = &cobra.Command{
	Use:   "command <command-name>",
	Short: "Create a new command file of your choice",
	Long: `Command takes one argument that is the name of user-defined command
(1 to 100 ASCII letters or digits, not starting with a digit).
Example :- "bean create command test" will create a command test in the commands folder.`,
	Args: cobra.ExactArgs(1),
	RunE: command,
}

func command(cmd *cobra.Command, args []string) error {
	p, err := loadProject()
	if err != nil {
		return err
	}

	name, err := commandName(args[0])
	if err != nil {
		return err
	}

	if err := generateFile(p, "command.go", filepath.Join("commands", name+".go"), Command{ProjectObject: *p, CommandName: name}); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "command with name %s and command file %s.go created\n", name, name)
	return nil
}

// commandName validates a command name and returns it in lower case. The command variable is `<name>Cmd`, so
// that a name like `type` or `fmt` does not clash with a keyword or an import.
func commandName(name string) (string, error) {
	if err := validateName("command", name); err != nil {
		return "", err
	}
	name = strings.ToLower(name)
	if name == "root" || token.IsKeyword(name) {
		return "", fmt.Errorf("the command name %q is reserved", name)
	}
	return name, nil
}
