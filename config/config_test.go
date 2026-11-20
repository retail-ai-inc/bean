package config

import (
	"os"
	"path/filepath"
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
