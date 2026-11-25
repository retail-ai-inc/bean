package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeRoute(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "routers"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "routers", "route.go"), []byte(content), 0o644))
	return dir
}

func readRoute(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "routers", "route.go"))
	require.NoError(t, err)
	return string(b)
}

func TestAddRouteEntry(t *testing.T) {
	tests := []struct {
		name  string
		route string
		want  string
	}{
		{
			name: "template layout",
			route: `package routers

type Handlers struct {
	exampleHdlr handlers.ExampleHandler
}

func Init() {
	hdlrs := &Handlers{
		exampleHdlr: handlers.NewExampleHandler(),
	}
	_ = hdlrs
}
`,
			want: `package routers

type Handlers struct {
	exampleHdlr handlers.ExampleHandler
	postHdlr    handlers.PostHandler // added by bean
}

func Init() {
	hdlrs := &Handlers{
		exampleHdlr: handlers.NewExampleHandler(),
		postHdlr:    handlers.NewPostHandler(), // added by bean
	}
	_ = hdlrs
}
`,
		},
		{
			name: "empty struct and literal",
			route: `package routers

type Handlers struct {
}

func Init() {
	hdlrs := &Handlers{
	}
	_ = hdlrs
}
`,
			want: `package routers

type Handlers struct {
	postHdlr handlers.PostHandler // added by bean
}

func Init() {
	hdlrs := &Handlers{
		postHdlr: handlers.NewPostHandler(), // added by bean
	}
	_ = hdlrs
}
`,
		},
		{
			name: "one line, no trailing comma",
			route: `package routers

type Handlers struct{ exampleHdlr handlers.ExampleHandler }

func Init() {
	hdlrs := &Handlers{exampleHdlr: handlers.NewExampleHandler()}
	_ = hdlrs
}
`,
			want: `package routers

type Handlers struct {
	exampleHdlr handlers.ExampleHandler
	postHdlr    handlers.PostHandler // added by bean
}

func Init() {
	hdlrs := &Handlers{exampleHdlr: handlers.NewExampleHandler(),
		postHdlr: handlers.NewPostHandler(), // added by bean
	}
	_ = hdlrs
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeRoute(t, tt.route)
			require.NoError(t, addRouteEntry(dir, "Handlers", "postHdlr handlers.PostHandler", "postHdlr: handlers.NewPostHandler()"))
			assert.Equal(t, tt.want, readRoute(t, dir))
		})
	}
}

func TestAddRouteEntry_NotFound(t *testing.T) {
	const route = "package routers\n\ntype Other struct{}\n"
	dir := writeRoute(t, route)
	err := addRouteEntry(dir, "Handlers", "postHdlr handlers.PostHandler", "postHdlr: handlers.NewPostHandler()")
	assert.ErrorContains(t, err, "type Handlers struct not found")
	assert.Equal(t, route, readRoute(t, dir), "nothing must be written")
}

func TestHandlerNameParts(t *testing.T) {
	for name, want := range map[string][]string{
		"example.com/demo/handlers.(*exampleHandler).JSONIndex-fm": {"(*exampleHandler)", "JSONIndex-fm"},
		"demo/handlers.(*exampleHandler).JSONIndex-fm":             {"(*exampleHandler)", "JSONIndex-fm"},
		"github.com/a/b.v2/handlers.(*h).Get-fm":                   {"(*h)", "Get-fm"},
		"example.com/demo/routers.Init.func1":                      {"Init", "func1"},
	} {
		got, ok := handlerNameParts(name)
		assert.True(t, ok, name)
		assert.Equal(t, want, got, name)
	}
	_, ok := handlerNameParts("example.com/demo/handlers.Index")
	assert.False(t, ok)
}

func TestReportFileProject(t *testing.T) {
	assert.Equal(t, "example.com_demo", reportFileProject("example.com/demo"))
	assert.Equal(t, "demo", reportFileProject("demo"))
}

func TestDockerName(t *testing.T) {
	assert.Equal(t, "demo", dockerName("example.com/demo"))
	assert.Equal(t, "my-app", dockerName("github.com/x/My_App"))
	assert.Equal(t, "app", dockerName("example.com/__"))
}

func TestProjectGoVersion(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module demo\n\ngo 1.26.10\n"), 0o644))
	assert.Equal(t, "1.26", projectGoVersion(dir))
}

func TestParseRouteList(t *testing.T) {
	out := `{"message":"route is not registered because its method is not listed in http.allowedMethod","severity":"WARNING"}
[{"method":"GET","name":"example.com/demo/handlers.ExampleHandler.JSONIndex-fm","path":"/json"}]
`
	routes, err := parseRouteList([]byte(out))
	require.NoError(t, err)
	assert.Equal(t, []routeListEntry{{Method: "GET", Name: "example.com/demo/handlers.ExampleHandler.JSONIndex-fm", Path: "/json"}}, routes)

	_, err = parseRouteList([]byte(`{"message":"only logs"}`))
	assert.Error(t, err)
}
