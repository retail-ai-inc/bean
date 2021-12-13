/*
 * Copyright The RAI Inc.
 * The RAI Authors
 *
 * *** PLEASE DO NOT DELETE THIS FILE. ***
 */

package interfaces

import "github.com/labstack/echo/v4"

/*
 * XXX: IMPORTANT - Write all your handler interface here. Please use unique name for your interface.
 */

type MyTestHandler interface {
	MyTestIndex(c echo.Context) error
}
