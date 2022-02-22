// Copyright The RAI Inc.
// The RAI Authors
package helpers

import (
	"io/ioutil"
	"math/rand"
	"time"

	str "github.com/retail-ai-inc/bean/string"
)

type CopyableMap map[string]interface{}
type CopyableSlice []interface{}

func GetRandomNumberFromRange(min, max int) int {

	rand.Seed(time.Now().UnixNano())

	n := min + rand.Intn(max-min+1)

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

// `IsFilesExistInDirectory` function will check the files (filesToCheck) exist in a specific diretory or not.
func IsFilesExistInDirectory(path string, filesToCheck []string) (bool, error) {

	var matchCount int

	numberOfFileToCheck := len(filesToCheck)

	if numberOfFileToCheck == 0 {
		return false, nil
	}

	files, err := ioutil.ReadDir(path)
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
