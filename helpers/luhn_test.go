// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Run test: go test -run "(TestIsValidLuhnNumber|TestCalculateLuhnNumber|TestGenerateLuhnNumber|TestGenerateLuhnNumberWithPrefix)" -v -count 1
package helpers

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidLuhnNumber(t *testing.T) {
	tests := []struct {
		number   string
		expectOK bool
	}{
		{"1234567812345670", true},
		{"1111222233334444", true},
		{"1111222233334441", false},
		{"49927398716", true},
		{"49927398717", false},
		{"1234567812345678", false},
		{"79927398710", false},
		{"79927398711", false},
		{"79927398712", false},
		{"79927398713", true},
		{"79927398714", false},
		{"79927398715", false},
		{"79927398716", false},
		{"79927398717", false},
		{"79927398718", false},
		{"79927398719", false},
		{"374652346956782346957823694857692364857368475368", true},
		{"374652346956782346957823694857692364857387456834", false},
		{"8", false},
		{"0", true},
	}
	for _, test := range tests {
		t.Run(test.number, func(t *testing.T) {
			got := IsValidLuhnNumber(test.number)
			assert.Equal(t, test.expectOK, got, "Luhn number validation failed for %s", test.number)
		})
	}
}

func TestCalculateLuhnNumber(t *testing.T) {
	tests := []struct {
		number     string
		checkDigit string
		expected   string
	}{
		{"123456781234567", "0", "1234567812345670"},
		{"111122223333444", "4", "1111222233334444"},
		{"7992739871", "3", "79927398713"},
		{"37465234695678234695782369485769236485736847536", "8", "374652346956782346957823694857692364857368475368"},
	}
	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			cd, got, err := CalculateLuhnNumber(test.number)
			require.NoError(t, err, "Unexpected error while calculating Luhn number for %s", test.number)
			assert.Equal(t, test.checkDigit, cd, "Unexpected Luhn digit for input %s", test.number)
			assert.Equal(t, test.expected, got, "Unexpected Luhn number for input %s", test.number)
			assert.True(t, IsValidLuhnNumber(got), "Generated Luhn number is invalid for input %s", test.number)
		})
	}
}

func TestGenerateLuhnNumber(t *testing.T) {
	tests := []struct {
		numberSize int
		sampleSize int
	}{
		{1 + 1, 100}, // minimum size is 2
		{10, 1000},
		{100, 1000},
		{1000, 1000},
	}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.numberSize), func(t *testing.T) {
			for i := 0; i < test.sampleSize; i++ {
				ln, err := GenerateLuhnNumber(test.numberSize)
				require.NoError(t, err, "Failed to generate Luhn number")
				assert.True(t, IsValidLuhnNumber(ln), "Generated Luhn number is invalid: %s", ln)
			}
		})
	}
}

func TestGenerateLuhnNumberWithPrefix(t *testing.T) {
	tests := []struct {
		numberSize int
		sampleSize int
	}{
		{1 + len("123"), 100}, // minimum size is more than prefix length
		{10, 1000},
		{100, 1000},
		{1000, 1000},
	}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.numberSize), func(t *testing.T) {
			for i := 0; i < test.sampleSize; i++ {
				ln, err := GenerateLuhnNumberWithPrefix("123", test.numberSize)
				require.NoError(t, err, "Failed to generate Luhn number with prefix")
				assert.True(t, IsValidLuhnNumber(ln), "Generated Luhn number with prefix is invalid: %s", ln)
			}
		})
	}
}
