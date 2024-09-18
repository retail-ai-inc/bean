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
	"math/rand"
	"strconv"
	"strings"
)

// IsValidLuhnNumber returns true if the provided string is a valid luhn number otherwise false.
func IsValidLuhnNumber(number string) bool {
	p := len(number) % 2
	sum, err := calculateLuhnSum(number, p)
	if err != nil {
		return false
	}
	// If the total modulo 10 is not equal to 0, then the number is invalid.

	return sum%10 == 0
}

// CalculateLuhnNumber returns luhn check digit and the provided string number with its luhn check digit appended.
func CalculateLuhnNumber(number string) (checkDigit string, luhnNumber string, err error) {
	p := (len(number) + 1) % 2
	sum, err := calculateLuhnSum(number, p)
	if err != nil {
		return "", "", err
	}

	checkDigitInt := (10 - (sum % 10)) % 10
	checkDigit = strconv.Itoa(int(checkDigitInt))

	luhnNumber = number + checkDigit
	return checkDigit, luhnNumber, nil
}

// GenerateLuhnNumber will generate a valid luhn number of the provided length
func GenerateLuhnNumber(length int) (string, error) {
	if length <= 1 {
		return "", fmt.Errorf("length must be greater than 1")
	}

	randNum := generateRandomNumber(length - 1)

	_, luhnNumber, _ := CalculateLuhnNumber(randNum)

	return luhnNumber, nil
}

// GenerateLuhnNumberWithPrefix will generate a valid luhn number of the provided length with prefix
func GenerateLuhnNumberWithPrefix(prefix string, length int) (string, error) {
	if len(prefix) >= length {
		return "", fmt.Errorf("prefix length (%d) must be less than total length (%d)", len(prefix), length)
	}

	randomPartLength := length - len(prefix) - 1
	randNum := prefix + generateRandomNumber(randomPartLength)

	_, luhnNumber, _ := CalculateLuhnNumber(randNum)

	return luhnNumber, nil
}

func calculateLuhnSum(number string, parity int) (int64, error) {
	var sum int64
	for i, r := range number {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid digit: %v", r)
		}

		// Convert ASCII to digit (0-9)
		d := int64(r - '0')

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
		sum += d
	}

	return sum, nil
}

// generateRandomNumber generates a random numeric string of the given length.
func generateRandomNumber(length int) string {

	var s strings.Builder
	s.Grow(length)
	for i := 0; i < length; i++ {
		s.WriteString(strconv.Itoa(rand.Intn(10))) // Random digit 0-9 in concurrent safe way
	}
	return s.String()
}
