package cmd

import (
	"bytes"
	"context"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestProject generates a project like `bean init example.com/demo` (without `go mod tidy`) in a temporary
// directory, which becomes the working directory.
func newTestProject(t *testing.T) string {
	t.Helper()
	tplDir, err := filepath.Abs("bean")
	require.NoError(t, err)
	InternalFS = os.DirFS(tplDir) // set by main from the embedded files
	dir := t.TempDir()

	p := &Project{Copyright: copyright, PkgPath: "example.com/demo", PkgName: "demo", RootDir: dir}
	p.RootFS, err = fs.Sub(InternalFS, "internal/project")
	require.NoError(t, err)
	require.NoError(t, fs.WalkDir(p.RootFS, ".", p.generateProjectFiles))

	t.Chdir(dir)
	return dir
}

// runBean runs the bean CLI in-process and returns its output and error.
func runBean(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.ExecuteContext(context.Background())

	// pflag appends to a slice flag already set by a previous run in the same process.
	if f := serviceCmd.Flags().Lookup("repo"); f != nil {
		_ = f.Value.(pflag.SliceValue).Replace(nil)
		f.Changed = false
	}
	return out.String(), err
}

func TestCreateCommands(t *testing.T) {
	dir := newTestProject(t)

	for _, args := range [][]string{
		{"create", "repo", "post"},
		{"create", "service", "post"},
		{"create", "handler", "post"},
		{"create", "service", "feed", "--repo", "post,user"},
		{"create", "command", "sync"},
		{"create", "dockerfile"},
	} {
		out, err := runBean(t, args...)
		require.NoError(t, err, "%v: %s", args, out)
	}

	route, err := os.ReadFile(filepath.Join(dir, "routers", "route.go"))
	require.NoError(t, err)
	for _, want := range []string{
		"postRepo: repositories.NewPostRepository(b.DBConn)",
		"userRepo: repositories.NewUserRepository(b.DBConn)",
		"postSvc: services.NewPostService(repos.postRepo)",
		"feedSvc: services.NewFeedService(repos.postRepo, repos.userRepo)",
		"postHdlr: handlers.NewPostHandler(svcs.postSvc)",
	} {
		assert.Contains(t, strings.Join(strings.Fields(string(route)), " "), want)
	}
	command, err := os.ReadFile(filepath.Join(dir, "commands", "sync.go"))
	require.NoError(t, err)
	assert.Contains(t, string(command), "rootCmd.AddCommand(syncCmd)")
	for _, f := range []string{"Dockerfile", "docker-compose.yml", ".air.toml"} {
		assert.FileExists(t, filepath.Join(dir, f))
	}

	// Every generated Go file parses and is gofmt'ed.
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = parser.ParseFile(token.NewFileSet(), path, src, parser.AllErrors)
		require.NoError(t, err, path)
		formatted, err := format.Source(src)
		require.NoError(t, err, path)
		assert.Equal(t, string(formatted), string(src), "%s is not gofmt'ed", path)
		return nil
	}))
}

func TestCreateCommands_Errors(t *testing.T) {
	newTestProject(t)

	_, err := runBean(t, "create", "handler", "post")
	require.NoError(t, err)

	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"create", "handler", "post"}, "handlers/post.go already exists"},
		{[]string{"create", "handler", ""}, "the handler name is empty"},
		{[]string{"create", "repo", "1post"}, "must not start with a digit"},
		{[]string{"create", "service", "user-profile"}, "ASCII letters or digits"},
		{[]string{"create", "command", "type"}, `the command name "type" is reserved`},
		{[]string{"create", "service", "feed", "--repo", "9x"}, "must not start with a digit"},
	} {
		_, err := runBean(t, tt.args...)
		assert.ErrorContains(t, err, tt.want, "%v", tt.args)
	}

	t.Chdir(t.TempDir())
	_, err = runBean(t, "create", "repo", "post")
	assert.ErrorIs(t, err, errNotBeanProject)
	require.NoError(t, os.WriteFile("env.json", []byte("{}"), 0o644))
	_, err = runBean(t, "init", "example.com/demo")
	assert.ErrorContains(t, err, "env.json already exists")
}
