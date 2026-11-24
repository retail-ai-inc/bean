package bean

import (
	"log/slog"
	"testing"

	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/retail-ai-inc/bean/v3/internal/gopool"
	blog "github.com/retail-ai-inc/bean/v3/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestConfig() *config.Config {
	conf := &config.Config{}
	conf.HTTP.BodyLimit = "1M"
	conf.HTTP.AllowedMethod = []string{"GET"}
	return conf
}

func useConfig(t *testing.T, conf *config.Config) {
	t.Helper()
	orig := config.Bean
	config.Bean = conf
	t.Cleanup(func() { config.Bean = orig })
}

func TestNew_ConfigNotLoaded(t *testing.T) {
	useConfig(t, nil)

	b, err := New()
	assert.Nil(t, b)
	assert.ErrorIs(t, err, ErrConfigNotLoaded)

	e, closeEcho, err := NewEcho()
	assert.Nil(t, e)
	assert.Nil(t, closeEcho)
	assert.ErrorIs(t, err, ErrConfigNotLoaded)
}

func TestNewEcho_InvalidConfigReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(c *config.Config)
		wantErr string
	}{
		{"trustIPRange", func(c *config.Config) { c.HTTP.TrustIPRange = []string{"not-an-ip"} }, "invalid http.trustIPRange"},
		{"bodyLimit", func(c *config.Config) { c.HTTP.BodyLimit = "lots" }, "invalid http.bodyLimit"},
		{"accessLog skip pattern", func(c *config.Config) {
			c.AccessLog.On = true
			c.AccessLog.SkipEndpoints = []string{"("}
		}, "accessLog.skipEndpoints"},
		{"prometheus skip pattern", func(c *config.Config) {
			c.Prometheus.On = true
			c.Prometheus.SkipEndpoints = []string{"("}
		}, "prometheus.skipEndpoints"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := validTestConfig()
			tt.modify(conf)
			useConfig(t, conf)

			e, closeEcho, err := NewEcho()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, e)
			assert.Nil(t, closeEcho)
		})
	}
}

func TestNewEcho_ReleasesPoolsOnError(t *testing.T) {
	size := 1
	conf := validTestConfig()
	conf.AsyncPool = []struct {
		Name       string
		Size       *int
		BlockAfter *int
	}{
		{Name: "first", Size: &size},
		{Name: "first", Size: &size}, // duplicated name: registration fails
	}
	useConfig(t, conf)

	_, _, err := NewEcho()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `failed to register goroutine pool "first"`)

	_, err = gopool.GetPool("first")
	assert.Error(t, err, "pools registered before the failure must be released")
}

func TestNew_ReinitializesLogger(t *testing.T) {
	useConfig(t, validTestConfig())

	b1, err := New()
	require.NoError(t, err)
	l1 := blog.Logger()

	b2, err := New()
	require.NoError(t, err)
	l2 := blog.Logger()

	assert.NotNil(t, l1)
	assert.NotSame(t, l1, l2, "a new bean instance must initialize a new logger")
	require.NoError(t, b1.Shutdown())
	require.NoError(t, b2.Shutdown())
}

func TestNewEcho_UsesBeanLoggerForEcho(t *testing.T) {
	conf := validTestConfig()
	conf.DebugLogLevel = "info"
	useConfig(t, conf)

	e, closeEcho, err := NewEcho()
	require.NoError(t, err)
	t.Cleanup(func() { _ = closeEcho() })

	require.NotNil(t, blog.Logger())
	assert.Same(t, blog.Logger().Logger, e.Logger, "c.Logger() and log.Logger() must be the same logger")
	assert.Equal(t, slog.LevelInfo, blog.Logger().Level())
}

func TestNewEcho_InvalidLogLevel(t *testing.T) {
	conf := validTestConfig()
	conf.DebugLogLevel = "verbose"
	useConfig(t, conf)

	_, _, err := NewEcho()
	assert.ErrorContains(t, err, "debugLogLevel")
}
