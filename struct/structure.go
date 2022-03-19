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
package structure

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// This function will help you to convert your object from struct to map[string]interface{} based
// on your JSON tag in your structs.
func StructToMap(data interface{}) (map[string]interface{}, error) {

	dataBytes, err := json.Marshal(data)

	if err != nil {

		return nil, err
	}

	mapData := make(map[string]interface{})

	err = json.Unmarshal(dataBytes, &mapData)

	if err != nil {

		return nil, err
	}

	return mapData, nil
}

// This function mainly developed to check `json` tag exist in an interface or not.
func IsTagExist(tag, key string, s interface{}) (bool, error) {

	rt := reflect.TypeOf(s).Elem()

	if rt.Kind() != reflect.Struct {

		return false, errors.New("Bad type")
	}

	for i := 0; i < rt.NumField(); i++ {

		f := rt.Field(i)

		// Use split to ignore tag "options".
		v := strings.Split(f.Tag.Get(key), ",")[0]

		if v == tag {

			return true, nil
		}
	}

	return false, nil
}
