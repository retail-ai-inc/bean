{{ .Copyright }}
package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"{{ .PkgPath }}/routers"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v2"
	"github.com/spf13/cobra"
)

var (
	// genTestCmd
	genTestCmd = &cobra.Command{
		Use:   "spec",
		Short: "generate spec json file",
		Long:  ``,
		Args:  cobra.NoArgs,
		Run:   genTest,
	}
)

func init() {
	genTestCmd.Flags().StringP("destination", "d", "./tests/spec.json", "Output file; defaults to `./tests/spec.json`.")
	genCmd.AddCommand(genTestCmd)
}

type spec struct {
	Name   string                  `json:"name"`
	Path   string                  `json:"path"`
	Method string                  `json:"method"`
	Header *map[string]interface{} `json:"header,omitempty"`
	Params *map[string]interface{} `json:"params,omitempty"`
	Query  *map[string]interface{} `json:"query,omitempty"`
	Body   *map[string]interface{} `json:"body,omitempty"`
}

func genTest(cmd *cobra.Command, args []string) {
	// Just initialize a plain sentry client option structure if sentry is on.
	if bean.BeanConfig.Sentry.On {
		bean.BeanConfig.Sentry.ClientOptions = &sentry.ClientOptions{
			Debug:       false,
			Dsn:         bean.BeanConfig.Sentry.Dsn,
			Environment: bean.BeanConfig.Environment,
		}
	}

	// Create a bean object
	b := bean.New()

	// Create an empty database dependency.
	b.DBConn = &bean.DBDeps{}

	// Init different routes.
	routers.Init(b)

	var specMap = make(map[string]map[string]spec)
	for _, r := range b.Echo.Routes() {
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