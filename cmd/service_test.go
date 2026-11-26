package cmd

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRepos(t *testing.T) {
	exists := func(names ...string) func(string) bool {
		return func(name string) bool {
			for _, n := range names {
				if n == name {
					return true
				}
			}
			return false
		}
	}

	tests := []struct {
		name     string
		service  string
		flags    []string
		exists   func(string) bool
		want     []ServiceRepo
		wantArgs string
	}{
		{
			name:    "no flag, no repository with the same name",
			service: "post",
			exists:  exists(),
		},
		{
			name:     "no flag, repository with the same name",
			service:  "post",
			exists:   exists("post"),
			want:     []ServiceRepo{{Upper: "Post", Lower: "post"}},
			wantArgs: "repos.postRepo",
		},
		{
			name:     "one flag replaces the repository with the same name",
			service:  "post",
			flags:    []string{"user"},
			exists:   exists("post"),
			want:     []ServiceRepo{{Upper: "User", Lower: "user"}},
			wantArgs: "repos.userRepo",
		},
		{
			name:     "several flags are normalized and deduplicated",
			service:  "post",
			flags:    []string{"User", "comment", " user ", ""},
			exists:   exists(),
			want:     []ServiceRepo{{Upper: "User", Lower: "user"}, {Upper: "Comment", Lower: "comment"}},
			wantArgs: "repos.userRepo, repos.commentRepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serviceRepos(tt.service, tt.flags, tt.exists)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantArgs, serviceConstructorArgs(got))
		})
	}

	_, err := serviceRepos("post", []string{"1user"}, exists())
	assert.Error(t, err)
}

// The generated service must compile against the constructor call written to `routers/route.go`.
func TestServiceTemplate(t *testing.T) {
	// `InternalFS` is set by the main package (embed), so read the template from the source tree.
	data, err := os.ReadFile("bean/internal/_tpl/service.go")
	require.NoError(t, err)
	tmpl, err := template.New("").Parse(string(data))
	require.NoError(t, err)

	tests := []struct {
		name       string
		repos      []ServiceRepo
		wantParams []string // constructor parameter names
		wantFields []string // struct field names
	}{
		{name: "no repository"},
		{
			name:       "one repository",
			repos:      []ServiceRepo{{Upper: "Post", Lower: "post"}},
			wantParams: []string{"postRepo"},
			wantFields: []string{"postRepository"},
		},
		{
			name:       "two repositories",
			repos:      []ServiceRepo{{Upper: "Post", Lower: "post"}, {Upper: "User", Lower: "user"}},
			wantParams: []string{"postRepo", "userRepo"},
			wantFields: []string{"postRepository", "userRepository"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tmpl.Execute(&buf, Service{
				ProjectObject:    Project{PkgPath: "example.com/app"},
				ServiceNameUpper: "Post",
				ServiceNameLower: "post",
				Repos:            tt.repos,
			}))

			file, err := parser.ParseFile(token.NewFileSet(), "post.go", buf.Bytes(), parser.ParseComments)
			require.NoError(t, err, buf.String())

			var imports []string
			for _, imp := range file.Imports {
				imports = append(imports, strings.Trim(imp.Path.Value, `"`))
			}
			if len(tt.repos) > 0 {
				assert.Contains(t, imports, "example.com/app/repositories")
			} else {
				assert.NotContains(t, imports, "example.com/app/repositories")
			}

			var params, fields []string
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncDecl:
					if n.Name.Name == "NewPostService" {
						for _, f := range n.Type.Params.List {
							for _, name := range f.Names {
								params = append(params, name.Name)
							}
						}
					}
				case *ast.TypeSpec:
					if st, ok := n.Type.(*ast.StructType); ok && n.Name.Name == "postService" {
						for _, f := range st.Fields.List {
							for _, name := range f.Names {
								fields = append(fields, name.Name)
							}
						}
					}
				}
				return true
			})
			assert.Equal(t, tt.wantParams, params, buf.String())
			assert.Equal(t, tt.wantFields, fields, buf.String())
		})
	}
}
