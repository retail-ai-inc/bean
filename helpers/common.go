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
	"math/rand"
	"os"
	"time"

	str "github.com/retail-ai-inc/bean/string"
)

type CopyableMap map[string]interface{}
type CopyableSlice []interface{}

// GetRandomNumberFromRange will generate and return a random integer from a range.
func GetRandomNumberFromRange(min, max int) int {

	// TODO: Use global seed and make go version as 1.20 minimum.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	n := min + rng.Intn(max-min+1)
	return n
}

// DeepCopy will create a deep copy of this map. The depth of this
// copy is all inclusive. Both maps and slices will be considered when
// making the copy.
// Keep in mind that the slices in the resulting map will be of type []interface{},
// so when using them, you will need to use type assertion to retrieve the value in the expected type.
// Reference: https://stackoverflow.com/questions/23057785/how-to-copy-a-map/23058707
func (m CopyableMap) DeepCopy() map[string]interface{} {

	result := map[string]interface{}{}

	for k, v := range m {
		// Handle maps
		mapvalue, isMap := v.(map[string]interface{})
		if isMap {
			result[k] = CopyableMap(mapvalue).DeepCopy()
			continue
		}

		// Handle slices
		slicevalue, isSlice := v.([]interface{})
		if isSlice {
			result[k] = CopyableSlice(slicevalue).DeepCopy()
			continue
		}

		result[k] = v
	}

	return result
}

// DeepCopy will create a deep copy of this slice. The depth of this
// copy is all inclusive. Both maps and slices will be considered when
// making the copy.
// Reference: https://stackoverflow.com/questions/23057785/how-to-copy-a-map/23058707
func (s CopyableSlice) DeepCopy() []interface{} {
	result := []interface{}{}

	for _, v := range s {
		// Handle maps
		mapvalue, isMap := v.(map[string]interface{})
		if isMap {
			result = append(result, CopyableMap(mapvalue).DeepCopy())
			continue
		}

		// Handle slices
		slicevalue, isSlice := v.([]interface{})
		if isSlice {
			result = append(result, CopyableSlice(slicevalue).DeepCopy())
			continue
		}

		result = append(result, v)
	}

	return result
}

// IsFilesExistInDirectory will check a file(s) is exist in a specific diretory or not.
// If you pass multiple files into `filesToCheck` slice then this function will chcek the existence
// of all those files. If one of the file doesn't exist, it will return `false`.
func IsFilesExistInDirectory(dir string, filesToCheck []string) (bool, error) {

	var matchCount int

	numberOfFileToCheck := len(filesToCheck)

	if numberOfFileToCheck == 0 {
		return false, nil
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, file := range files {

		if !file.IsDir() {
			isMatch := str.Contains(filesToCheck, file.Name())
			if isMatch {
				matchCount++
			}
		}
	}

	if numberOfFileToCheck == matchCount {
		return true, nil
	}

	return false, nil
}
