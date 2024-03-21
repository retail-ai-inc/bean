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
		Run:   genSpec,
	}
)

func init() {
	genSpecCmd.Flags().StringP("destination", "d", "./tests/spec.json", "Output file; defaults to `./tests/spec.json`.")
	genCmd.AddCommand(genSpecCmd)
	rootCmd.AddCommand(genCmd)
}

func genSpec(cmd *cobra.Command, args []string) {
	var out bytes.Buffer
	routeCmd := exec.Command("go", "run", "main.go", "route", "list", "-j")
	routeCmd.Stdout = &out
	routeCmd.Stderr = os.Stderr
	if err := routeCmd.Run(); err != nil {
		log.Fatalf("Failed to execute route list command: %v", err)
	}
	var routeList []struct {
		Method string `json:"method"`
		Name   string `json:"name"`
		Path   string `json:"path"`
	}
	err := json.Unmarshal(out.Bytes(), &routeList)
	if err != nil {
		log.Fatalf("Failed to unmarshal route list: %v", err)
	}

	var specMap = make(map[string]map[string]spec)
	for _, r := range routeList {
		if strings.Contains(r.Name, "glob..func1") {
			continue
		}

		names := strings.SplitN(r.Name, ".", 2)
		names = strings.Split(names[1], ".")

		if len(names) != 2 {
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
		log.Fatalf("Failed to get destination arguments: %v", err)
	}

	var writer = os.Stdout
	if destination != "" {
		if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
			log.Fatalf("Unable to create directory: %v", err)
		}
		specJSONFile, err := os.Create(destination)
		if err != nil {
			log.Fatalf("Failed opening destination file: %v", err)
		}
		defer specJSONFile.Close()
		writer = specJSONFile
	}

	bs, err := json.MarshalIndent(specMap, "", "\t")
	if err != nil {
		log.Fatalf("Failed to marshal specMap: %v", err)
	}

	_, err = io.CopyBuffer(writer, bytes.NewReader(bs), nil)
	if err != nil {
		log.Fatalf("Failed to write specJSONFile: %v", err)
	}
	log.Printf("Generate %q completed.\n", destination)
}
