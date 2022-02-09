/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package helpers

import (
	"runtime"
	"runtime/debug"
)

// Returns the current version, only support module mode binaries.
func CurrVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		return bi.Main.Version
	}
	return ""
}

// Returns the name of the current running function.
func CurrFuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}
