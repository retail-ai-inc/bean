package log

import (
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

// This is a global variable to hold the debug logger so that we can log data from service, repository or anywhere.
var logger echo.Logger
var mu sync.Mutex

func New() echo.Logger {
	if logger == nil {
		mu.Lock()
		logger = log.New("echo")
		mu.Unlock()
	}

	return logger
}

func Set(l echo.Logger) {
	mu.Lock()
	defer mu.Unlock()
	logger = l
}

// The bean Logger to have debug log from anywhere.
func Logger() echo.Logger {
	return logger
}
