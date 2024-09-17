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
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	asciiZero = 48
	asciiTen  = 57
)

// IsValidLuhnNumber returns true if the provided string is a valid luhn number otherwise false.
func IsValidLuhnNumber(number string) bool {
	p := len(number) % 2
	sum, err := calculateLuhnSum(number, p)
	if err != nil {
		return false
	}

	// If the total modulo 10 is not equal to 0, then the number is invalid.
	if sum%10 != 0 {
		return false
	}

	return true
}

// CalculateLuhnNumber returns luhn check digit and the provided string number with its luhn check digit appended.
func CalculateLuhnNumber(number string) (string, string, error) {
	p := (len(number) + 1) % 2
	sum, err := calculateLuhnSum(number, p)
	if err != nil {
		return "", "", nil
	}

	luhn := sum % 10
	if luhn != 0 {
		luhn = 10 - luhn
	}

	// If the total modulo 10 is not equal to 0, then the number is invalid.
	return strconv.FormatInt(luhn, 10), fmt.Sprintf("%s%d", number, luhn), nil
}

// GenerateLuhnNumber will generate a valid luhn number of the provided length
func GenerateLuhnNumber(length int) string {
	r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

	var s strings.Builder
	for i := 0; i < length-1; i++ {
		s.WriteString(strconv.Itoa(r.Intn(9)))
	}

	_, res, _ := CalculateLuhnNumber(s.String()) // ignore error because this will always be valid

	return res
}

// GenerateLuhnNumberWithPrefix will generate a valid luhn number of the provided length with prefix
func GenerateLuhnNumberWithPrefix(prefix string, length int) string {
	r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

	var s strings.Builder
	s.WriteString(prefix)
	length -= len(prefix)

	for i := 0; i < length-1; i++ {
		s.WriteString(strconv.Itoa(r.Intn(9)))
	}

	_, res, _ := CalculateLuhnNumber(s.String())
	return res
}

func calculateLuhnSum(number string, parity int) (int64, error) {
	var sum int64
	for i, d := range number {
		if d < asciiZero || d > asciiTen {
			return 0, errors.New("invalid digit")
		}

		d = d - asciiZero
		// Double the value of every second digit.
		if i%2 == parity {
			d *= 2
			// If the result of this doubling operation is greater than 9.
			if d > 9 {
				// The same final result can be found by subtracting 9 from that result.
				d -= 9
			}
		}

		// Take the sum of all the digits.
		sum += int64(d)
	}

	return sum, nil
}
