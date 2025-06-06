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

package helpers

import (
	"strings"
	"testing"
)

func TestDeterministicUUID(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name          string
		seeder1       string
		seeder2       string
		expectEqual   bool
		removeHyphens bool
	}{
		{
			name:          "Same seeder produces same UUID with hyphens",
			seeder1:       "test-seeder",
			seeder2:       "test-seeder",
			expectEqual:   true,
			removeHyphens: false,
		},
		{
			name:          "Same seeder produces same UUID without hyphens",
			seeder1:       "test-seeder",
			seeder2:       "test-seeder",
			expectEqual:   true,
			removeHyphens: true,
		},
		{
			name:          "Different seeders produce different UUIDs with hyphens",
			seeder1:       "test-seeder-1",
			seeder2:       "test-seeder-2",
			expectEqual:   false,
			removeHyphens: false,
		},
		{
			name:          "Different seeders produce different UUIDs without hyphens",
			seeder1:       "test-seeder-1",
			seeder2:       "test-seeder-2",
			expectEqual:   false,
			removeHyphens: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uuid1, err := DeterministicUUID(tc.seeder1, tc.removeHyphens)
			if err != nil {
				t.Fatalf("unexpected error for seeder1: %v", err)
			}

			uuid2, err := DeterministicUUID(tc.seeder2, tc.removeHyphens)
			if err != nil {
				t.Fatalf("unexpected error for seeder2: %v", err)
			}

			if (uuid1 == uuid2) != tc.expectEqual {
				t.Errorf("expected equality: %v, but got uuid1=%v and uuid2=%v", tc.expectEqual, uuid1, uuid2)
			}

			// Verify hyphen removal
			if tc.removeHyphens {
				if strings.Contains(uuid1, "-") || strings.Contains(uuid2, "-") {
					t.Errorf("expected UUIDs without hyphens, but got uuid1=%v, uuid2=%v", uuid1, uuid2)
				}
			} else {
				if !strings.Contains(uuid1, "-") || !strings.Contains(uuid2, "-") {
					t.Errorf("expected UUIDs with hyphens, but got uuid1=%v, uuid2=%v", uuid1, uuid2)
				}
			}
		})
	}
}
