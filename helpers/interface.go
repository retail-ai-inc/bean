package helpers

import (
	"reflect"
	"strconv"

	"github.com/pkg/errors"
)

func ConvertInterfaceToArray(value interface{}) interface{} {
	value = indirect(value)
	vType := reflect.ValueOf(value)
	switch vType.Kind() {
	case reflect.Array, reflect.Slice:
		return value
	default:
		return []interface{}{value}
	}
}

func ConvertInterfaceToBool(value interface{}) (bool, error) {
	value = indirect(value)
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return strconv.ParseBool(strconv.FormatInt(int64(v), 10))
	case int8:
		return strconv.ParseBool(strconv.FormatInt(int64(v), 10))
	case int16:
		return strconv.ParseBool(strconv.FormatInt(int64(v), 10))
	case int32:
		return strconv.ParseBool(strconv.FormatInt(int64(v), 10))
	case int64:
		return strconv.ParseBool(strconv.FormatInt(v, 10))
	case uint:
		return strconv.ParseBool(strconv.FormatUint(uint64(v), 10))
	case uint8:
		return strconv.ParseBool(strconv.FormatUint(uint64(v), 10))
	case uint16:
		return strconv.ParseBool(strconv.FormatUint(uint64(v), 10))
	case uint32:
		return strconv.ParseBool(strconv.FormatUint(uint64(v), 10))
	case uint64:
		return strconv.ParseBool(strconv.FormatUint(v, 10))
	case float32:
		return strconv.ParseBool(strconv.FormatFloat(float64(v), 'f', -1, 64))
	case float64:
		return strconv.ParseBool(strconv.FormatFloat(v, 'f', -1, 64))
	default:
		return false, errors.New("wrong parameter type")
	}
}

func ConvertInterfaceToFloat(value interface{}) (float64, error) {
	value = indirect(value)
	switch v := value.(type) {
	case string:
		return strconv.ParseFloat(v, 64)
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, errors.New("wrong parameter type")
	}
}

func ConvertInterfaceToString(value interface{}) (string, error) {
	value = indirect(value)
	switch v := value.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", errors.New("wrong parameter type")
	}
}

// From html/template/content.go
// Copyright 2011 The Go Authors. All rights reserved.
// indirect returns the value, after dereferencing as many times
// as necessary to reach the base type (or nil).
func indirect(i interface{}) interface{} {
	if i == nil {
		return nil
	}
	if t := reflect.TypeOf(i); t.Kind() != reflect.Ptr {
		// Avoid creating a reflect.Value if it's not a pointer.
		return i
	}
	v := reflect.ValueOf(i)
	for v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	return v.Interface()
}
