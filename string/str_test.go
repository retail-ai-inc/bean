package str

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubstring(t *testing.T) {
	assert.Equal(t, "日本", Substring("日本語", 0, 2))
	assert.Equal(t, "日本語", Substring("日本語", 0, 5), "j beyond the end is clamped to the number of runes")
	assert.Equal(t, "", Substring("日本語", 3, 5))
	assert.Equal(t, "", Substring("日本語", 5, 9), "i beyond the end in runes (but not in bytes)")
	assert.Equal(t, "本語", Substring("日本語", 1, -1))
	assert.Equal(t, "hello", Substring("hello world", 0, 5))
}

func TestPadToLength(t *testing.T) {
	// Same results as before for valid input.
	assert.Equal(t, "005", LeftPadToLength("5", "0", 3))
	assert.Equal(t, "babx", LeftPadToLength("x", "ab", 4))
	assert.Equal(t, "345", LeftPadToLength("12345", "0", 3))
	assert.Equal(t, "500", RightPadToLength("5", "0", 3))
	assert.Equal(t, "xaba", RightPadToLength("x", "ab", 4))
	assert.Equal(t, "123", RightPadToLength("12345", "0", 3))

	// These panicked.
	assert.Equal(t, "ababa", RightPadToLength("", "ab", 5))
	assert.Equal(t, "babab", LeftPadToLength("", "ab", 5))
	assert.Equal(t, "x", LeftPadToLength("x", "", 3))
	assert.Equal(t, "", RightPadToLength("x", "0", -1))
	assert.Equal(t, "", LeftPadToLength("x", "0", 0))
}
