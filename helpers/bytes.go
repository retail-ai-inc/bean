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
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	byteSizeBinary  = regexp.MustCompile(`(?i)^(-?\d+(?:\.\d+)?)\s?([KMGTPE]iB?)$`)
	byteSizeDecimal = regexp.MustCompile(`(?i)^(-?\d+(?:\.\d+)?)\s?([KMGTPE]B?|B?)$`)
)

// ParseByteSize parses a human readable size such as "1M", "512KB", "1.5MiB" or "100" into bytes.
// `K`, `M`, `G`, ... (with or without `B`) are decimal (1000 based); `Ki`, `Mi`, `Gi`, ... (with or without `B`)
// are binary (1024 based). It is case-insensitive and follows `github.com/labstack/gommon/bytes.Parse`,
// which bean v2 used for `http.bodyLimit`.
func ParseByteSize(value string) (int64, error) {
	if parts := byteSizeBinary.FindStringSubmatch(value); len(parts) == 3 {
		return scaleByteSize(parts[1], strings.TrimSuffix(strings.ToUpper(parts[2]), "IB"), 1024)
	}
	if parts := byteSizeDecimal.FindStringSubmatch(value); len(parts) == 3 {
		return scaleByteSize(parts[1], strings.TrimSuffix(strings.ToUpper(parts[2]), "B"), 1000)
	}
	return 0, fmt.Errorf("invalid byte size %q", value)
}

func scaleByteSize(number, unit string, base float64) (int64, error) {
	n, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q: %w", number, err)
	}
	unit = strings.TrimSuffix(unit, "I") // "Mi" -> "M"
	multiplier := 1.0
	for _, u := range []string{"K", "M", "G", "T", "P", "E"} {
		multiplier *= base
		if unit == u {
			return int64(n * multiplier), nil
		}
	}
	return int64(n), nil // no unit or "B"
}
