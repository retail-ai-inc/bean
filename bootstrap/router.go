/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package bootstrap

import (
	"bean/routers"

	"github.com/labstack/echo/v4"
)

// InitRouter registers all the endpoints to the router.
func InitRouter(e *echo.Echo) {

	// Initialize all endpoint routing.
	routers.Init(e)
}
