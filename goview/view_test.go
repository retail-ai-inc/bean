package goview

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEngine(t *testing.T, disableCache bool) *ViewEngine {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"index.html":            `index {{.user}}`,
		"footer.html":           `footer {{.user}}`,
		"templates/master.html": `MASTER[{{template "index" .}}|{{include "footer"}}]`,
		"templates/admin.html":  `ADMIN[{{template "index" .}}]`,
		"broken.html":           `{{.missing.field}}`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	cfg := DefaultConfig
	cfg.Root = root
	cfg.Funcs = map[string]any{}
	cfg.DisableCache = disableCache
	return New(cfg)
}

func render(t *testing.T, e *ViewEngine, name string, data any) string {
	t.Helper()
	var sb strings.Builder
	require.NoError(t, e.RenderWriter(&sb, name, data))
	return sb.String()
}

// `include` must render with the data of its own render, even when renders run concurrently on cached templates.
func TestViewEngine_ConcurrentIncludeUsesOwnData(t *testing.T) {
	for _, disableCache := range []bool{false, true} {
		t.Run(fmt.Sprintf("disableCache=%v", disableCache), func(t *testing.T) {
			e := newTestEngine(t, disableCache)

			var wg sync.WaitGroup
			for i := range 200 {
				wg.Go(func() {
					user := fmt.Sprintf("user%d", i)
					var sb strings.Builder
					err := e.RenderWriter(&sb, "index", map[string]any{"template": "", "user": user})
					assert.NoError(t, err)
					assert.Equal(t, fmt.Sprintf("MASTER[index %s|footer %s]", user, user), sb.String())
				})
			}
			wg.Wait()
		})
	}
}

// The layout of one render must not leak into the next one, and the cache must keep one parsed set per layout.
func TestViewEngine_LayoutIsPerRender(t *testing.T) {
	e := newTestEngine(t, false)

	assert.Equal(t, "index a", render(t, e, "index", map[string]any{"user": "a"}))
	assert.Equal(t, "ADMIN[index b]", render(t, e, "index", map[string]any{"template": "templates/admin", "user": "b"}))
	assert.Equal(t, "MASTER[index c|footer c]", render(t, e, "index", map[string]any{"template": "", "user": "c"}))
	assert.Equal(t, "index d", render(t, e, "index", map[string]any{"user": "d"}))
}

func TestViewEngine_AcceptsM(t *testing.T) {
	e := newTestEngine(t, false)
	assert.Equal(t, "index m", render(t, e, "index", M{"user": "m"}))
}

func TestViewEngine_RenderErrorWritesNothing(t *testing.T) {
	e := newTestEngine(t, false)

	rec := httptest.NewRecorder()
	err := e.Render(rec, 404, "broken", map[string]any{"missing": 1})
	require.Error(t, err)
	assert.False(t, rec.Flushed)
	assert.Empty(t, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Content-Type"))

	rec = httptest.NewRecorder()
	require.NoError(t, e.Render(rec, 404, "index", map[string]any{"user": "x"}))
	assert.Equal(t, 404, rec.Code)
	assert.Equal(t, "index x", rec.Body.String())
	assert.Equal(t, HTMLContentType[0], rec.Header().Get("Content-Type"))
}

func BenchmarkViewEngine_Render(b *testing.B) {
	t := &testing.T{}
	e := newTestEngine(t, false)
	data := map[string]any{"template": "", "user": "bench"}
	b.ReportAllocs()
	for b.Loop() {
		var sb strings.Builder
		if err := e.RenderWriter(&sb, "index", data); err != nil {
			b.Fatal(err)
		}
	}
}
