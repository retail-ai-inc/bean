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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	handlerCmd.DisableFlagsInUseLine = true
	createCmd.AddCommand(handlerCmd)
}

type Handler struct {
	ProjectObject    Project
	HandlerNameUpper string
	HandlerNameLower string
	ServiceExists    bool
}

// handlerCmd represents the handler command
var handlerCmd = &cobra.Command{
	Use:   "handler <handler-name>",
	Short: "Create a new handler file of your choice",
	Long: `Command takes one argument that is the name of user-defined handler
(1 to 100 ASCII letters or digits, not starting with a digit).
Example :- "bean create handler post" will create a handler Post in handlers folder.
If the service services/post.go exists, it is injected into the handler.`,
	Args: cobra.ExactArgs(1),
	RunE: handler,
}

func handler(cmd *cobra.Command, args []string) error {
	p, err := loadProject()
	if err != nil {
		return err
	}

	name, err := componentName("handler", args[0])
	if err != nil {
		return err
	}
	lower := strings.ToLower(name)

	data := Handler{
		ProjectObject:    *p,
		HandlerNameUpper: name,
		HandlerNameLower: lower,
		// If a service with the same name exists, inject it.
		ServiceExists: fileExists(p.RootDir, "services", lower+".go"),
	}
	if err := generateFile(p, "handler.go", filepath.Join("handlers", lower+".go"), data); err != nil {
		return err
	}

	svcArg := ""
	if data.ServiceExists {
		svcArg = "svcs." + lower + "Svc"
	}
	if err := wireRoute(p.RootDir, "Handlers",
		lower+"Hdlr handlers."+name+"Handler",
		lower+"Hdlr: handlers.New"+name+"Handler("+svcArg+")",
	); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "handler with name %s and handler file %s.go created\n", name, lower)
	return nil
}
