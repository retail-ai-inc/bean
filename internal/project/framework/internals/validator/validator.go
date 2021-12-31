/**#bean*/ /*#bean.replace({{ .Copyright }})**/

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
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

var validate *validator.Validate

// ValidationError implements the builtin `error` interface and extends custom output function.
type ValidationError struct {
	Err error
}

// CustomValidator implements the Echo#Validator interface.
type CustomValidator struct {
	validate *validator.Validate
}

func BindCustomValidator(e *echo.Echo, initializer func(c echo.Context, vd *validator.Validate)) {

	c := e.AcquireContext()
	c.Reset(nil, nil)

	validate = validator.New()

	// XXX: IMPORTANT - Add your customize validation functions.
	if initializer != nil {
		initializer(c, validate)
	}

	e.Validator = &CustomValidator{validate: validate}
}

// Validate implements the `Echo#Validator.Validate` function.
func (cv *CustomValidator) Validate(data interface{}) error {

	err := cv.validate.Struct(data)
	if err != nil {

		// This check is only needed when your code could produce an invalid value for
		// validation such as interface with nil value. Most including myself do not usually
		// have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {

			panic(err)
		}

		return &ValidationError{err}
	}

	return nil
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

		case "testUserIdValidation":

			errorCollection = append(errorCollection, map[string]string{"field": fieldname, "code": fieldname + "_invalid_user_id"})

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
