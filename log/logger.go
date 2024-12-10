package log

import "github.com/labstack/echo/v4"

// This is a global variable to hold the debug logger so that we can log data from service, repository or anywhere.
var logger echo.Logger

func Set(l echo.Logger) {
	logger = l
}

// The bean Logger to have debug log from anywhere.
func Logger() echo.Logger {
	return logger
}
