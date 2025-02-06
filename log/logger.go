package log

import (
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

// This is a global variable to hold the debug logger so that we can log data from service, repository or anywhere.
var (
	logger echo.Logger
	once   sync.Once
)

func New() echo.Logger {
	once.Do(func() {
		logger = log.New("echo")
	})
	return logger
}

// Set sets the logger. It is not thread-safe.
func Set(l echo.Logger) {
	logger = l
}

// Logger returns the logger.
func Logger() echo.Logger {
	return logger
}
