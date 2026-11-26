package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseByteSize(t *testing.T) {
	tests := map[string]int64{
		"100":    100,
		"10B":    10,
		"1K":     1000,
		"512KB":  512000,
		"1M":     1000000,
		"1m":     1000000,
		"1 M":    1000000,
		"1MB":    1000000,
		"1.5M":   1500000,
		"1G":     1000000000,
		"1Ki":    1024,
		"1KiB":   1024,
		"1Mi":    1048576,
		"1MiB":   1048576,
		"2gib":   2 * 1024 * 1024 * 1024,
		"1T":     1000000000000,
		"-1":     -1,
		"0.5KiB": 512,
	}
	for in, want := range tests {
		got, err := ParseByteSize(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, in := range []string{"", "lots", "1X", "M", "1 MBB"} {
		_, err := ParseByteSize(in)
		assert.Error(t, err, in)
	}
}
