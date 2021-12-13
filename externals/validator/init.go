/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package validator

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func Init(c echo.Context, vd *validator.Validate) {

	/*
	 * Here you can add your own validator functions.
	 */
	// userbarcodeid(c, validate)

}
