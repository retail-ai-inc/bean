{{ .Copyright }}
package validator

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func TestUserIdValidation(c echo.Context, vd *validator.Validate) {

	// Register a custom validation for TRIAL user's barcode id.
	_ = validate.RegisterValidation("testUserIdValidation", func(fl validator.FieldLevel) bool {

		// TRIAL has 2 sets of barcode id starting with `296` or `299`.
		if fl.Field().String()[0:3] == "296" {

			return true

		} else if fl.Field().String()[0:3] == "299" { // Test TRIAL barcode id

			return true

		} else {

			return false
		}
	})
}
