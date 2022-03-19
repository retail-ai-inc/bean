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
package str

import (
	"encoding/base64"
	"math/rand"
	"net/url"
	"strings"
	"unicode"
)

// IsBlank Checks if a string is whitespace, empty ("").
func IsBlank(str string) bool {

	return len(strings.TrimSpace(str)) == 0
}

// IsNotBlank Checks if a string is not empty (""), not null and not whitespace only.
func IsNotBlank(str string) bool {

	return !IsBlank(str)
}

// IsEmpty Checks if a string is whitespace, empty ("").
func IsEmpty(str string) bool {

	return len(str) == 0
}

// IsNotEmpty Checks if a string is not empty ("").
func IsNotEmpty(str string) bool {

	return !IsEmpty(str)
}

// IsEqualsAny tests whether a string equals any string provided.
func IsEqualsAny(val string, vals ...string) bool {

	for _, v := range vals {

		if val == v {

			return true
		}
	}

	return false
}

// IsValidUrl tests a string to determine if it is a well-structured url or not.
func IsValidUrl(urlString string) bool {

	_, err := url.ParseRequestURI(urlString)
	if err != nil {
		return false
	}

	u, err := url.Parse(urlString)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}

// DefaultIfNil Returns either the passed in string, or if the string is
// empty (""), the value of default string.
func DefaultIfNil(str, defaultStr string) string {

	if IsEmpty(str) {

		return defaultStr
	}

	return str
}

// DefaultIfBlank Returns either the passed in string, or if the string is
// whitespace, empty (""), the value of default string.
func DefaultIfBlank(str, defaultStr string) string {

	if IsBlank(str) {

		return defaultStr
	}

	return str
}

func StringToPointer(str string) *string {

	s := &str

	return s
}

// De-referencing *string to string
func DerefString(s *string) string {

	if s != nil {

		return *s
	}

	return ""
}

// Substring Returns a substring of str in range(i, j).
// str.Substring("manju", 0, 1) output: m
func Substring(str string, i, j int) string {

	runes := []rune(str)

	if i >= len(str) {

		return ""
	}

	if j > len(str) {

		j = len(str)
	}

	ld := i >= 0
	rd := j >= 0

	if ld && rd {

		if j <= i {

			return ""
		}

		return string(runes[i:j])
	}

	if ld {
		return string(runes[i:])
	} else if rd {
		return string(runes[:j])
	}

	return str
}

// `IsMatchAllSubstrings` this function will return `true` only if all substrings matches the `str`.
func IsMatchAllSubstrings(str string, subs ...string) bool {

	isCompleteMatch := true

	for _, sub := range subs {

		if !strings.Contains(str, sub) {
			isCompleteMatch = false
			break
		}
	}

	return isCompleteMatch
}

// `MatchSubstringsInAString` this function will return bool and number of matches. It is useful when you need
// to know at least one match found and do something. `matches` will return 0 or greater than 0.
func MatchAllSubstringsInAString(str string, subs ...string) (bool, int) {

	matches := 0
	isCompleteMatch := true

	for _, sub := range subs {

		if strings.Contains(str, sub) {
			matches += 1

		} else {
			isCompleteMatch = false
		}
	}

	return isCompleteMatch, matches
}

// This function just check an element is present in a slice or not. It's a linear search (O(n)) but after finding
// the result it will return immidiately.
func Contains(s []string, str string) bool {

	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

// This function will add extra `padStr` on the left of `s`. FYI, If `overallLen` is 20 and the actual length of
// `s` is 25 then this function will remove first 5 characters from the left side of `s` to make the size equal to 20.
func LeftPadToLength(s string, padStr string, overallLen int) string {

	padCountInt := int(1 + ((overallLen - len(padStr)) / len(padStr)))

	var retStr = strings.Repeat(padStr, padCountInt) + s

	return retStr[(len(retStr) - overallLen):]
}

func RightPadToLength(s string, padStr string, overallLen int) string {

	padCountInt := int(1 + ((overallLen - len(padStr)) / len(padStr)))

	var retStr = s + strings.Repeat(padStr, padCountInt)

	return retStr[:overallLen]
}

func AlphaNumericRandomString(length int) string {

	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, length)

	for i := range b {

		b[i] = letter[rand.Intn(len(letter))]
	}

	return string(b)
}

// GenerateRandomBytes returns securely generated random bytes. It will return an error if the system's secure random
// number generator fails to function correctly, in which case the caller should not continue.
func GenerateRandomBytes(n int) ([]byte, error) {

	b := make([]byte, n)

	_, err := rand.Read(b)

	// Note that err == nil only if we read len(b) bytes.
	if err != nil {

		return nil, err
	}

	return b, nil
}

// GenerateRandomString returns a URL-safe, base64 encoded securely generated random string.
// It will return an error if the system's secure random number generator fails to function correctly,
// in which case the caller should not continue.
func GenerateRandomString(length int, isSpecialCharacter bool) (string, error) {

	b, err := GenerateRandomBytes(length)

	if err != nil {

		return "", err
	}

	// This will keep `-` and `_`
	if isSpecialCharacter {

		return base64.RawURLEncoding.EncodeToString(b), nil

	} else {

		encodedString := ""

		// A safe concurrent way of multiple string replacement which replace/remove  `-` and `_`.
		re := strings.NewReplacer("_", "", "-", "")

		for len(encodedString) < length {

			strlength := len(encodedString)

			size := length - strlength

			bytes, err := GenerateRandomBytes(size)

			if err != nil {

				return "", err
			}

			s := re.Replace(base64.RawURLEncoding.EncodeToString(bytes))

			encodedString += Substring(s, 0, size)
		}

		return encodedString, nil
	}
}

// Everything in Go is passed by value, slices too. But a slice value is a header, describing a
// contiguous section of a backing array, and a slice value only contains a pointer to the array where
// the elements are actually stored. The slice value does not include its elements (unlike arrays).
// So when you pass a slice to a function, a copy will be made from this header, including the pointer,
// which will point to the same backing array.
// Modifying the elements of the slice implies modifying the elements of the backing array, and so all slices
// which share the same backing array will "observe" the change.

//`RemoveLeadingZerosFromSlice` this fuction will remove leading "0" from a string of slice.
func RemoveLeadingZerosFromSlice(slice []string) []string {

	if len(slice) == 0 {

		return slice
	}

	for i, v := range slice {

		slice = append(slice[:i], strings.TrimLeft(v, "0"))
	}

	return slice
}

// `ToSnakeCase` is a function to convert a sting into snake case.
func ToSnakeCase(s string) string {

	var res = make([]rune, 0, len(s))
	var p = '_'

	for i, r := range s {

		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {

			res = append(res, '_')

		} else if unicode.IsUpper(r) && i > 0 {

			if unicode.IsLetter(p) && !unicode.IsUpper(p) || unicode.IsDigit(p) {

				res = append(res, '_', unicode.ToLower(r))

			} else {

				res = append(res, unicode.ToLower(r))
			}

		} else {

			res = append(res, unicode.ToLower(r))
		}

		p = r
	}

	return string(res)
}
