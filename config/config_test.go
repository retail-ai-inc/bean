package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestConfig(t *testing.T, accessLog string) (*Config, error) {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)

	file := filepath.Join(t.TempDir(), "env.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"accessLog": `+accessLog+`}`), 0o600))

	return LoadConfig(file)
}

func TestLoadConfigMaskFields(t *testing.T) {
	cfg, err := loadTestConfig(t, `{"maskFields": ["password", "token"]}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"password", "token"}, cfg.AccessLog.MaskFields)
}

func TestLoadConfigRemovedBodyDumpMaskParam(t *testing.T) {
	tests := []struct {
		name      string
		accessLog string
		wantErr   bool
	}{
		{"non-empty old key fails", `{"bodyDumpMaskParam": ["password"]}`, true},
		{"non-empty old key fails even with new key", `{"maskFields": ["password"], "bodyDumpMaskParam": ["password"]}`, true},
		{"empty old key is ignored", `{"bodyDumpMaskParam": [], "maskFields": ["password"]}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadTestConfig(t, tt.accessLog)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "rename it to accessLog.maskFields")
				assert.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{"password"}, cfg.AccessLog.MaskFields)
		})
	}
}

func loadEnv(t *testing.T, env string) *Config {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)

	file := filepath.Join(t.TempDir(), "env.json")
	require.NoError(t, os.WriteFile(file, []byte(env), 0o600))
	cfg, err := LoadConfig(file)
	require.NoError(t, err)
	return cfg
}

func TestLoadConfigHTTPDefaults(t *testing.T) {
	// The whole "http" block omitted.
	cfg := loadEnv(t, `{"projectName": "demo"}`)
	assert.Equal(t, DefaultHTTPHost, cfg.HTTP.Host)
	assert.Equal(t, DefaultHTTPPort, cfg.HTTP.Port)
	assert.Equal(t, DefaultHTTPBodyLimit, cfg.HTTP.BodyLimit)
	assert.True(t, cfg.HTTP.KeepAlive)

	// Partially configured: the other keys still get their defaults.
	cfg = loadEnv(t, `{"http": {"port": "9000"}}`)
	assert.Equal(t, "9000", cfg.HTTP.Port)
	assert.Equal(t, DefaultHTTPBodyLimit, cfg.HTTP.BodyLimit)
	assert.True(t, cfg.HTTP.KeepAlive)

	// Explicit values win, including false for keepAlive.
	cfg = loadEnv(t, `{"http": {"host": "127.0.0.1", "port": "8080", "bodyLimit": "2M", "keepAlive": false}}`)
	assert.Equal(t, "127.0.0.1", cfg.HTTP.Host)
	assert.Equal(t, "8080", cfg.HTTP.Port)
	assert.Equal(t, "2M", cfg.HTTP.BodyLimit)
	assert.False(t, cfg.HTTP.KeepAlive)
}

// Projects may read env.json with viper directly instead of LoadConfig: the defaults registered by
// SetDefaults (called when the package is initialized) must apply too.
func TestHTTPDefaultsWithViperDirectly(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	SetDefaults()

	viper.SetConfigType("json")
	require.NoError(t, viper.ReadConfig(strings.NewReader(`{"projectName": "demo"}`)))
	cfg := &Config{}
	require.NoError(t, viper.Unmarshal(cfg))

	assert.Equal(t, DefaultHTTPPort, cfg.HTTP.Port)
	assert.Equal(t, DefaultHTTPBodyLimit, cfg.HTTP.BodyLimit)
	assert.True(t, cfg.HTTP.KeepAlive)
}
