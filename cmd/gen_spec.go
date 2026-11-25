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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

type spec struct {
	Name   string                  `json:"name"`
	Path   string                  `json:"path"`
	Method string                  `json:"method"`
	Header *map[string]interface{} `json:"header,omitempty"`
	Params *map[string]interface{} `json:"params,omitempty"`
	Query  *map[string]interface{} `json:"query,omitempty"`
	Body   *map[string]interface{} `json:"body,omitempty"`
}

var (
	genCmd = &cobra.Command{
		Use:   "gen [command]",
		Short: "Generate some files.",
		Long:  ``,
	}

	// genTestCmd
	genSpecCmd = &cobra.Command{
		Use:   "spec",
		Short: "generate spec.json file",
		Long:  ``,
		Args:  cobra.NoArgs,
		RunE:  genSpec,
	}
)

func init() {
	genSpecCmd.Flags().StringP("destination", "d", "./tests/spec.json", "Output file; defaults to `./tests/spec.json`.")
	genCmd.AddCommand(genSpecCmd)
	rootCmd.AddCommand(genCmd)
}

func genSpec(cmd *cobra.Command, args []string) error {
	var out bytes.Buffer
	routeCmd := exec.Command("go", "run", "main.go", "route", "list", "-j")
	routeCmd.Stdout = &out
	routeCmd.Stderr = os.Stderr
	if err := routeCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute route list command: %w", err)
	}
	routeList, err := parseRouteList(out.Bytes())
	if err != nil {
		return fmt.Errorf("failed to unmarshal route list: %w", err)
	}

	var specMap = make(map[string]map[string]spec)
	for _, r := range routeList {
		if strings.Contains(r.Name, "glob..func1") {
			continue
		}

		names, ok := handlerNameParts(r.Name)
		if !ok {
			continue
		}

		if unicode.IsLower([]rune(names[1])[0]) {
			continue
		}

		name := strings.TrimSuffix(names[1], "-fm")

		var s = spec{
			Name:   name,
			Path:   r.Path,
			Method: r.Method,
			Header: &map[string]interface{}{},
			Params: &map[string]interface{}{},
			Query:  &map[string]interface{}{},
		}
		if s.Method == http.MethodGet || s.Method == http.MethodDelete {
			s.Body = nil
		} else {
			s.Body = &map[string]interface{}{}
		}

		if _, ok := specMap[names[0]]; ok {
			specMap[names[0]][s.Name] = s
		} else {
			specMap[names[0]] = map[string]spec{
				s.Name: s,
			}
		}
	}

	destination, err := cmd.Flags().GetString("destination")
	if err != nil {
		return fmt.Errorf("failed to get destination arguments: %w", err)
	}

	var writer = os.Stdout
	if destination != "" {
		if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
			return fmt.Errorf("unable to create directory: %w", err)
		}
		specJSONFile, err := os.Create(destination)
		if err != nil {
			return fmt.Errorf("failed opening destination file: %w", err)
		}
		defer func(f *os.File) {
			if err := f.Close(); err != nil {
				log.Printf("Failed close destination file:%v", err)
			}
		}(specJSONFile)
		writer = specJSONFile
	}

	bs, err := json.MarshalIndent(specMap, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal specMap: %w", err)
	}

	_, err = io.CopyBuffer(writer, bytes.NewReader(bs), nil)
	if err != nil {
		return fmt.Errorf("failed to write specJSONFile: %w", err)
	}
	log.Printf("Generate %q completed.\n", destination)
	return nil
}

// handlerNameParts splits a route name like `example.com/demo/handlers.(*exampleHandler).JSONIndex-fm` (the
// handler's function name) into the receiver `(*exampleHandler)` and the method `JSONIndex-fm`. The package path
// is removed first: it may contain dots (e.g. a domain).
func handlerNameParts(routeName string) ([]string, bool) {
	name := routeName
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	_, name, ok := strings.Cut(name, ".") // remove the package name
	if !ok {
		return nil, false
	}
	parts := strings.Split(name, ".")
	if len(parts) != 2 || parts[1] == "" {
		return nil, false
	}
	return parts, true
}

type routeListEntry struct {
	Method string `json:"method"`
	Name   string `json:"name"`
	Path   string `json:"path"`
}

// parseRouteList returns the routes printed by `route list -j`. The application logs go to stdout too (e.g. a
// warning for a route whose method is not allowed), as JSON objects: the route list is the JSON array among them.
func parseRouteList(out []byte) ([]routeListEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.New("no route list in the output")
			}
			return nil, err
		}
		if trimmed := bytes.TrimSpace(value); len(trimmed) == 0 || trimmed[0] != '[' {
			continue // a log line
		}
		var routes []routeListEntry
		if err := json.Unmarshal(value, &routes); err != nil {
			return nil, err
		}
		return routes, nil
	}
}
