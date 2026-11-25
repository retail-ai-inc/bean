package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJitterBackoff_SmallMin(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Equal(t, time.Duration(0), JitterBackoff(0, time.Second, 0))
		assert.Equal(t, time.Nanosecond, JitterBackoff(time.Nanosecond, time.Second, 0))
	})
	d := JitterBackoff(time.Millisecond, time.Second, 3)
	assert.GreaterOrEqual(t, d, time.Millisecond)
}

// Integers above 2^53 must keep their value.
func TestStructStripTags_KeepsLargeIntegers(t *testing.T) {
	v := struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}{ID: 9007199254740993, Name: " <b>x</b> "}

	require.NoError(t, StructStripTags(&v, true))
	assert.Equal(t, int64(9007199254740993), v.ID)
	assert.Equal(t, "&lt;b&gt;x&lt;/b&gt;", v.Name)
}
