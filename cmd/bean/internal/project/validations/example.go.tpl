{{ .Copyright }}
package validations

import "github.com/go-playground/validator/v10"

func Example(v *validator.Validate) error {
	return v.RegisterValidation("example", func(fl validator.FieldLevel) bool {
		return fl.Field().String() == "example"
	})
}
