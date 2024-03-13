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
	"fmt"
	"go/token"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/golang/mock/mockgen/model"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/v2"
	"github.com/spf13/cobra"
	toolsimports "golang.org/x/tools/imports"
)

const (
	SPEC_DEFAULT_PATH = "./tests"
	SPEC_NAME         = "spec"
)

var (
	// genTestCmd
	genTestCmd = &cobra.Command{
		Use:   "gen",
		Short: "",
		Long:  ``,
		Args:  cobra.NoArgs,
		RunE:  genTest,
	}
)

func init() {
	genTestCmd.Flags().StringP("source", "s", "", "(source mode) Input Go source file; enables source mode.")
	genTestCmd.Flags().StringP("destination", "d", "", "Output file; defaults to stdout.")
	TestCmd.AddCommand(genTestCmd)
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

func genTest(cmd *cobra.Command, args []string) error {
	destination, err := cmd.Flags().GetString("destination")
	if err != nil {
		return errors.WithStack(err)
	}

	var specMap = make(map[string]map[string]spec)
	source, err := cmd.Flags().GetString("source")
	if err != nil {
		return errors.WithStack(err)
	}
	if source == "" {
		source = path.Join(SPEC_DEFAULT_PATH, SPEC_NAME+".json")
	}

	if _, err := os.Stat(source); os.IsNotExist(err) {
		return errors.WithStack(err)
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return errors.WithStack(err)
	}

	err = json.Unmarshal(data, &specMap)
	if err != nil {
		return errors.WithStack(err)
	}

	specDest := path.Join(SPEC_DEFAULT_PATH, SPEC_NAME, SPEC_NAME+"_interface.go")
	if err := os.MkdirAll(filepath.Dir(specDest), os.ModePerm); err != nil {
		log.Fatalf("Unable to create directory: %v", err)
	}
	interfaceFile, err := os.Create(specDest)
	if err != nil {
		log.Fatalf("Failed opening destination file: %v", err)
	}
	defer interfaceFile.Close()

	iface, err := genSpecInterface(specMap, SPEC_NAME)
	if err != nil {
		return errors.WithStack(err)
	}
	log.Printf("Generate %q completed.\n", specDest)

	_, err = interfaceFile.Write(iface.Output())
	if err != nil {
		return errors.WithStack(err)
	}

	if destination == "" {
		destination = path.Join(SPEC_DEFAULT_PATH, SPEC_NAME, SPEC_NAME+".go")
	}

	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		log.Fatalf("Unable to create directory: %v", err)
	}
	templateFile, err := os.Create(destination)
	if err != nil {
		log.Fatalf("Failed opening destination file: %v", err)
	}
	defer templateFile.Close()

	source = specDest
	pkg, err := sourceMode(source)
	if err != nil {
		return errors.WithStack(err)
	}

	g := new(generator)
	g.filename = SPEC_NAME
	g.destination = destination
	g.specMap = specMap

	outputPackageName := sanitize(pkg.Name)

	var outputPackagePath string
	dstPath, err := filepath.Abs(filepath.Dir(destination))
	if err == nil {
		pkgPath, err := parsePackageImport(dstPath)
		if err == nil {
			outputPackagePath = pkgPath
		} else {
			log.Println("Unable to infer -self_package from destination file path:", err)
		}
	} else {
		log.Println("Unable to determine destination file path:", err)
	}

	if err := g.Generate(pkg, outputPackageName, outputPackagePath); err != nil {
		log.Fatalf("Failed generating mock: %v", err)
	}
	if _, err := templateFile.Write(g.Output()); err != nil {
		log.Fatalf("Failed writing to destination: %v", err)
	}
	log.Printf("Generate %q completed.\n", destination)

	return nil
}

func genSpecInterface(specs map[string]map[string]spec, filename string) (*generator, error) {
	g := new(generator)
	g.filename = filename
	g.destination = path.Join(SPEC_DEFAULT_PATH, SPEC_NAME)
	g.p("// Code generated by bean test gen. DO NOT EDIT.")
	g.p("// Source: %v", g.filename)
	g.p("")
	g.p("// Package %v is a generated http client package.", filename)
	g.p("package %v", filename)
	g.p("")
	g.p("import (")
	g.in()
	g.p("%q", "context")
	g.p("")
	g.p("%q", "github.com/go-resty/resty/v2")

	g.out()
	g.p(")")
	g.p("")

	for name, spec := range specs {
		g.p("type %s interface{", name)
		g.in()
		for _, s := range spec {
			var sts = []string{"ctx context.Context"}
			st := "%s(" + strings.Join(sts, ",") + ") *resty.Response"
			g.p(st, s.Name)
		}
		g.out()
		g.p("}")
		g.p("")
	}
	return g, nil
}

type generator struct {
	buf             bytes.Buffer
	indent          string
	mockNames       map[string]string // may be empty
	filename        string            // may be empty
	destination     string            // may be empty
	copyrightHeader string

	packageMap map[string]string // map from import path to package name

	specMap map[string]map[string]spec //
}

func (g *generator) p(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, g.indent+format+"\n", args...)
}

func (g *generator) in() {
	g.indent += "\t"
}

func (g *generator) out() {
	if len(g.indent) > 0 {
		g.indent = g.indent[0 : len(g.indent)-1]
	}
}

// sanitize cleans up a string to make a suitable package name.
func sanitize(s string) string {
	t := ""
	for _, r := range s {
		if t == "" {
			if unicode.IsLetter(r) || r == '_' {
				t += string(r)
				continue
			}
		} else {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				t += string(r)
				continue
			}
		}
		t += "_"
	}
	if t == "_" {
		t = "x"
	}
	return t
}

func (g *generator) Generate(pkg *model.Package, outputPkgName string, outputPackagePath string) error {
	g.p("// Code generated by bean test gen. DO NOT EDIT.")
	g.p("// Source: %v", g.filename)
	g.p("")

	// Get all required imports, and generate unique names for them all.
	im := pkg.Imports()

	// so only import if any of the mocked interfaces have methods.
	for _, intf := range pkg.Interfaces {
		if len(intf.Methods) > 0 {
			break
		}
	}

	// Sort keys to make import alias generation predictable
	sortedPaths := make([]string, len(im))
	x := 0
	for pth := range im {
		sortedPaths[x] = pth
		x++
	}
	sort.Strings(sortedPaths)

	packagesName := createPackageMap(sortedPaths)

	g.packageMap = make(map[string]string, len(im))
	localNames := make(map[string]bool, len(im))
	for _, pth := range sortedPaths {
		base, ok := packagesName[pth]
		if !ok {
			base = sanitize(path.Base(pth))
		}

		// Local names for an imported package can usually be the basename of the import path.
		// A couple of situations don't permit that, such as duplicate local names
		// (e.g. importing "html/template" and "text/template"), or where the basename is
		// a keyword (e.g. "foo/case").
		// try base0, base1, ...
		pkgName := base
		i := 0
		for localNames[pkgName] || token.Lookup(pkgName).IsKeyword() {
			pkgName = base + strconv.Itoa(i)
			i++
		}

		// Avoid importing package if source pkg == output pkg
		if pth == pkg.PkgPath && outputPackagePath == pkg.PkgPath {
			continue
		}

		g.packageMap[pth] = pkgName
		localNames[pkgName] = true
	}

	g.p("// Package %v is a generated interface package.", outputPkgName)
	g.p("package %v", outputPkgName)
	g.p("")
	g.p("import (")
	g.in()
	for pkgPath, pkgName := range g.packageMap {
		if pkgPath == outputPackagePath {
			continue
		}
		g.p("%v %q", pkgName, pkgPath)
	}
	for _, pkgPath := range pkg.DotImports {
		g.p(". %q", pkgPath)
	}

	g.p("%q", "testing")
	g.p("%q", "github.com/stretchr/testify/assert")
	g.p("%q", "github.com/retail-ai-inc/bean/v2/test")

	g.out()
	g.p(")")

	g.p(
		`type CommonParams struct {
					CartUUID     string
					ClientID     string
                    ClientSecret string
                }`,
	)

	for _, intf := range pkg.Interfaces {
		if err := g.GenerateInterface(intf, outputPackagePath); err != nil {
			return err
		}
	}

	return nil
}

// The name of the mock type to use for the given interface identifier.
func (g *generator) mockName(typeName string) string {
	trimTypeName := strings.TrimSuffix(typeName, "Handler")
	if trimTypeName == typeName {
		return typeName
	}
	return FirstLower(strings.TrimSuffix(typeName, "Handler") + "Client")
}

func (g *generator) restoreMockName(mockName string) string {
	trimMockName := strings.TrimSuffix(mockName, "Client")
	if trimMockName == mockName {
		return mockName
	}
	return FirstUpper(trimMockName + "Handler")
}

func FirstUpper(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func FirstLower(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func (g *generator) GenerateInterface(intf *model.Interface, outputPackagePath string) error {
	mockType := g.mockName(intf.Name)

	g.p("")
	g.p("// %v is a mock of %v interface.", mockType, intf.Name)
	g.p("type %v struct {", mockType)
	g.in()
	g.p("t           *testing.T")
	g.p("Params      CommonParams")
	g.p("http        *resty.Client")
	g.p("endPoint    string")
	g.p("accessToken string")
	g.out()
	g.p("}")
	g.p("")

	g.p("// New%v creates a new http client instance.", FirstUpper(mockType))
	g.p("func New%v(t *testing.T, params CommonParams) *%v {", FirstUpper(mockType), mockType)
	g.in()
	g.p("t.Helper()")
	g.p("")
	g.p("client := &%v{", mockType)
	g.in()
	g.p("t:           t,")
	g.p("Params:      params,")
	g.p("http:        test.NewHTTPClientWithoutRetry(),")
	g.p("endPoint:    \"http://\"+viper.GetString(\"%v.http.host\")+\":\"+viper.GetString(\"%v.http.port\"),", "manju", "manju")
	g.p("accessToken: \"\",")
	g.out()
	g.p("}")
	g.p("")
	g.p("return client")
	g.out()
	g.p("}")

	g.GenerateMethods(mockType, intf, outputPackagePath)

	return nil
}

type byMethodName []*model.Method

func (b byMethodName) Len() int           { return len(b) }
func (b byMethodName) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byMethodName) Less(i, j int) bool { return b[i].Name < b[j].Name }

func (g *generator) GenerateMethods(mockType string, intf *model.Interface, pkgOverride string) {
	sort.Sort(byMethodName(intf.Methods))
	for _, m := range intf.Methods {
		g.p("")
		err := g.GenerateMethod(mockType, m, pkgOverride)
		if err != nil {
			bean.Logger().Error(err)
			continue
		}
	}
}

func makeArgString(argNames, argTypes []string) string {
	args := make([]string, len(argNames))
	for i, name := range argNames {
		// specify the type only once for consecutive args of the same type
		if i+1 < len(argTypes) && argTypes[i] == argTypes[i+1] {
			args[i] = name
		} else {
			args[i] = name + " " + argTypes[i]
		}
	}
	return strings.Join(args, ", ")
}

// GenerateMethod generates a mock method implementation.
// If non-empty, pkgOverride is the package in which unqualified types reside.
func (g *generator) GenerateMethod(mockType string, m *model.Method, pkgOverride string) error {
	argNames := g.getArgNames(m)
	argTypes := g.getArgTypes(m, pkgOverride)
	argString := makeArgString(argNames, argTypes)

	var rets []string
	for _, p := range m.Out {
		rets = append(rets, p.Type.String(g.packageMap, pkgOverride))
	}
	retString := strings.Join(rets, ", ")
	if len(rets) > 1 {
		retString = "(" + retString + ")"
	}
	if retString != "" {
		retString = " " + retString
	}

	ia := newIdentifierAllocator(argNames)
	idRecv := ia.allocateIdentifier("c")

	g.p("// %v mocks base method.", m.Name)
	g.p("func (%v *%v) %v(%v)%v {", idRecv, mockType, m.Name, argString, retString)
	g.in()
	g.p("resp,err:=c.http.R().SetContext(ctx).")
	g.p("SetAuthToken(c.accessToken).")
	g.p(`SetHeaders(map[string]string{
			"Content-Type": "application/json",
            "Client-Id":c.Params.ClientID,
			"Client-Secret":c.Params.ClientSecret,
			"UUID":c.Params.CartUUID,
		}).`)

	mName := g.restoreMockName(mockType)
	if specs, ok := g.specMap[mName]; ok {
		if s, ok := specs[m.Name]; ok {
			if s.Header != nil && len(*s.Header) > 0 {
				for k, v := range *s.Header {
					g.p("SetHeader(%q,%q).", k, v)
				}
			}
			if s.Params != nil && len(*s.Params) > 0 {
				for k, v := range *s.Params {
					s.Path = strings.ReplaceAll(s.Path, fmt.Sprintf("/:%s", k), fmt.Sprintf("/%v", v))
				}
			}
			if s.Query != nil && len(*s.Query) > 0 {
				s.Path = s.Path + "?"
				var qs []string
				for k, v := range *s.Query {
					qs = append(qs, fmt.Sprintf("%s=%v", k, v))
				}
				s.Path = s.Path + strings.Join(qs, "&")
			}
			switch s.Method {
			case http.MethodGet:
				g.p(`Get(c.endPoint + "%v")`, s.Path)
			case http.MethodPost:
				if s.Body != nil && len(*s.Body) > 0 {
					g.p("SetBody(%v).", fmt.Sprintf("%#v", *s.Body))
				}

				g.p(`Post(c.endPoint + "%v")`, s.Path)
			case http.MethodDelete:
				g.p(`Delete(c.endPoint + "%v")`, s.Path)
			default:
				return errors.New("not support")
			}
		}
	} else {
		panic(fmt.Errorf("%q not exist", strings.Join([]string{g.restoreMockName(mockType), m.Name}, ".")))
	}
	g.p("")
	g.p("assert.NoError(c.t,err)")
	g.p("")
	g.p("return resp")
	g.out()
	g.p("}")
	return nil
}

func (g *generator) getArgNames(m *model.Method) []string {
	argNames := make([]string, len(m.In))
	for i, p := range m.In {
		name := p.Name
		if name == "" || name == "_" {
			name = fmt.Sprintf("arg%d", i)
		}
		argNames[i] = name
	}
	if m.Variadic != nil {
		name := m.Variadic.Name
		if name == "" {
			name = fmt.Sprintf("arg%d", len(m.In))
		}
		argNames = append(argNames, name)
	}
	return argNames
}

func (g *generator) getArgTypes(m *model.Method, pkgOverride string) []string {
	argTypes := make([]string, len(m.In))
	for i, p := range m.In {
		argTypes[i] = p.Type.String(g.packageMap, pkgOverride)
	}
	if m.Variadic != nil {
		argTypes = append(argTypes, "..."+m.Variadic.Type.String(g.packageMap, pkgOverride))
	}
	return argTypes
}

type identifierAllocator map[string]struct{}

func newIdentifierAllocator(taken []string) identifierAllocator {
	a := make(identifierAllocator, len(taken))
	for _, s := range taken {
		a[s] = struct{}{}
	}
	return a
}

func (o identifierAllocator) allocateIdentifier(want string) string {
	id := want
	for i := 2; ; i++ {
		if _, ok := o[id]; !ok {
			o[id] = struct{}{}
			return id
		}
		id = want + "_" + strconv.Itoa(i)
	}
}

// Output returns the generator's output, formatted in the standard Go style.
func (g *generator) Output() []byte {
	if path.Ext(g.destination) == ".go" {
		src, err := toolsimports.Process(g.destination, g.buf.Bytes(), nil)
		if err != nil {
			log.Fatalf("Failed to format generated source code: %s\n%s", err, g.buf.String())
		}
		return src
	}

	return g.buf.Bytes()
}
