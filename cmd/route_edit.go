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
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
)

const addedByBean = " // added by bean"

// addRouteEntry wires a generated component into `routers/route.go`: it adds `field` (e.g.
// `postHdlr handlers.PostHandler`) to the struct type `typeName` (e.g. `Handlers`) and `element` (e.g.
// `postHdlr: handlers.NewPostHandler()`) to the composite literal of that type (e.g. `hdlrs := &Handlers{...}`).
//
// The file is parsed, so the layout of route.go does not matter (an empty struct, braces on one line...), and the
// result is gofmt'ed. Nothing is written if the type or the literal is not found or the result does not compile.
func addRouteEntry(projectDir, typeName, field, element string) error {
	path := filepath.Join(projectDir, "routers", "route.go")
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return err
	}

	var structType *ast.StructType
	var literal *ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.TypeSpec:
			if st, ok := n.Type.(*ast.StructType); ok && n.Name.Name == typeName && structType == nil {
				structType = st
			}
		case *ast.CompositeLit:
			if id, ok := n.Type.(*ast.Ident); ok && id.Name == typeName && literal == nil {
				literal = n
			}
		}
		return true
	})
	if structType == nil {
		return fmt.Errorf("type %s struct not found in %s", typeName, path)
	}
	if literal == nil {
		return fmt.Errorf("composite literal %s{...} not found in %s", typeName, path)
	}

	type insertion struct {
		offset int
		text   string
	}
	offset := func(p token.Pos) int { return fset.Position(p).Offset }

	// beforeClosing returns the text to insert right before a closing brace: on its own line.
	beforeClosing := func(closing token.Pos, text string) insertion {
		off := offset(closing)
		prefix := "\n"
		if line := bytes.TrimRight(src[:off], " \t"); len(line) > 0 && line[len(line)-1] == '\n' {
			prefix = "" // the brace is already on its own line
		}
		return insertion{off, prefix + text + "\n"}
	}

	insertions := []insertion{beforeClosing(structType.Fields.Closing, field+addedByBean)}
	// The elements of a literal are separated by commas: add one after the last element if it has none.
	if n := len(literal.Elts); n > 0 {
		last := offset(literal.Elts[n-1].End())
		if !bytes.Contains(stripComments(src[last:offset(literal.Rbrace)]), []byte(",")) {
			insertions = append(insertions, insertion{last, ","})
		}
	}
	insertions = append(insertions, beforeClosing(literal.Rbrace, element+","+addedByBean))

	// Build the result in offset order; at the same offset, keep the order of the list.
	sort.SliceStable(insertions, func(i, j int) bool { return insertions[i].offset < insertions[j].offset })
	var out []byte
	prev := 0
	for _, ins := range insertions {
		out = append(out, src[prev:ins.offset]...)
		out = append(out, ins.text...)
		prev = ins.offset
	}
	out = append(out, src[prev:]...)

	formatted, err := format.Source(out)
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", path, err)
	}
	return os.WriteFile(path, formatted, 0o644)
}

// stripComments removes the `//` comments of src (a piece of Go code between two tokens).
func stripComments(src []byte) []byte {
	var out []byte
	for _, line := range bytes.Split(src, []byte("\n")) {
		if i := bytes.Index(line, []byte("//")); i >= 0 {
			line = line[:i]
		}
		out = append(out, line...)
	}
	return out
}

// wireRoute calls `addRouteEntry`; on failure, the error says what to add to routers/route.go by hand (the
// generated file is kept).
func wireRoute(projectDir, typeName, field, element string) error {
	if err := addRouteEntry(projectDir, typeName, field, element); err != nil {
		return fmt.Errorf("could not update routers/route.go: %w\nadd it by hand:\n\tin `type %s struct`: %s\n\tin `&%s{...}`: %s,",
			err, typeName, field, typeName, element)
	}
	return nil
}
