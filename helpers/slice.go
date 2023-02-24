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

// HasStringInSlice will tell whether slice contains str or false
// If a modifier func is provided, it is called with the slice item before the comparation:
// 	modifier := func(s string) string {
// 		if s == "cc" {
// 			return "ee"
// 		}
// 		return s
// 	}

// 	if !slice.HasStringInSlice(src, "ee", modifier) {
// 	}
func HasStringInSlice(slice []string, str string, modifier func(str string) string) bool {

	for _, i := range slice {

		if str == i {
			return true
		}

		if modifier != nil && modifier(i) == str {
			return true
		}
	}

	return false
}

// FindStringInSlice returns the smallest index at which str == slice[index], or -1 if there is no such index.
func FindStringInSlice(slice []string, str string) int {

	for index, item := range slice {

		if str == item {
			return index
		}
	}

	return -1
}

// DeleteStringFromSlice will delete a string from a specific index of a slice.
func DeleteStringFromSlice(slice []string, index int) []string {
	return append(slice[:index], slice[index+1:]...)
}
