{{ .Copyright }}
package validator

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func TestUserIdValidation(c echo.Context, vd *validator.Validate) {

	// This is sample validation function to show how you should write your custom validation functon.
	_ = validate.RegisterValidation("testUserIdValidation", func(fl validator.FieldLevel) bool {

		// Check first 3 digits starting with `296` or `299`.
		if fl.Field().String()[0:3] == "296" {

			return true

		} else if fl.Field().String()[0:3] == "299" {

			return true

		} else {

			return false
		}
	})
}
