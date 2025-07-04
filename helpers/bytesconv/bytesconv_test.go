package bytesconv

import (
	"testing"
)

func TestBytesToString(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Normal ASCII",
			input:    []byte("aa"),
			expected: "aa",
		},
		{
			name:     "Empty slice",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "Nil slice",
			input:    nil,
			expected: "",
		},
		{
			name:     "Unicode characters",
			input:    []byte("你好"),
			expected: "你好",
		},
		{
			name:     "Special symbols",
			input:    []byte("!@#$%^&*()"),
			expected: "!@#$%^&*()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BytesToString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStringToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{
			name:     "Normal String",
			input:    "aaa",
			expected: []byte("aaa"),
		},
		{
			name:     "Empty String",
			input:    "",
			expected: []byte{},
		},
		{
			name:     "Chinese Characters",
			input:    "你好",
			expected: []byte("你好"),
		},
		{
			name:     "Special Characters",
			input:    "!@#$%^&*()",
			expected: []byte("!@#$%^&*()"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToBytes(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
