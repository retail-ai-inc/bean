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

// A sample POST data validation error response looks like as follows:
// 	{
// 	    "errorCode": "200001",
// 	    "errors": [
// 	        {
// 	            "field": "name",
// 	            "code": "name_required"
// 	        }
// 	    ]
// 	}

package validator

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// ValidatorFunc is a custom validator function you can define and add to the validator.
type ValidatorFunc func(*validator.Validate) error

// ValidationError implements the builtin `error` interface and extends custom output function.
type ValidationError struct {
	Err error
}

// DefaultValidator implements the Echo#Validator interface.
type DefaultValidator struct {
	Validator *validator.Validate
}

var _ echo.Validator = (*DefaultValidator)(nil)

// Validate implements the `Echo#Validator.Validate` function.
func (dv *DefaultValidator) Validate(data interface{}) error {
	if err := dv.Validator.Struct(data); err != nil {
		// Checking any invalid data passed to the validator.
		if err, ok := err.(*validator.InvalidValidationError); ok {
			panic(err)
		}
		return &ValidationError{err}
	}
	return nil
}

func NewDefaultValidator(v *validator.Validate) (*DefaultValidator, error) {
	if v == nil {
		return nil, errors.New("validator is nil")
	}

	return &DefaultValidator{Validator: v}, nil
}

// ErrCollection formats the validation errors and return it as a slice.
func (ve *ValidationError) ErrCollection() []map[string]string {

	errorCollection := []map[string]string{}

	for _, err := range ve.Err.(validator.ValidationErrors) {

		// Lowercase the field name
		fieldname := strings.ToLower(err.StructField())

		switch err.Tag() {

		case "required":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_required"})

		case "email":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_not_email"})

		case "uuid":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_not_uuid"})

		case "max":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_max_" + err.Param()})

		case "min":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_min_" + err.Param()})

		case "len":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_length_" + err.Param()})

		case "eqfield":

			// <field2>_not_same_<field1>
			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_not_same_" + strings.ToLower(err.Param())})

		case "alpha":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_alpha_only"})

		case "numeric":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_numeric_only"})

		case "number":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_number_only"})

		case "eq":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_eq_" + err.Param()})

		case "ne":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_ne_" + err.Param()})

		case "gt":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_gt_" + err.Param()})

		case "lt":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_lt_" + err.Param()})

		case "gte":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_gte_" + err.Param()})

		case "lte":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_lte_" + err.Param()})

		case "oneof":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_oneof_" + err.Param()})

		default:

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_invalid"})
		}
	}

	return errorCollection
}

// Error implements the builtin `error.Error` function.
func (ve *ValidationError) Error() string {
	return ve.Err.Error()
}

// Unwrap implements the builtin `error.Unwrap` function.
func (ve *ValidationError) Unwrap() error {
	return ve.Err
}
