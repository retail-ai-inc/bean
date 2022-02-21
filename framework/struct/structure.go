// Copyright The RAI Inc.
// The RAI Authors
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
