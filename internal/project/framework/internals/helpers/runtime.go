package helpers

import (
	"runtime"
)

// Gets the name of the current running function.
func CurrFuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}
