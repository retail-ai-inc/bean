/*
 * Copyright The RAI Inc.
 * The RAI Authors
 *
 * Safe way to execute `go routine` without crashing the parent process while having a `panic`.
 */

package async

import (
	"fmt"
	"reflect"

	"bean/framework/internals/global"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

/*
 * `Execute` provides a safe way to execute a  function asynchronously, recovering if they panic
 * and provides all error stack aiming to facilitate fail causes discovery.
 */
func Execute(fn func(ctx echo.Context)) {

	go func() {
		// Acquire a context from global echo instance and reset it to avoid race condition.
		c := global.EchoInstance.AcquireContext()
		c.Reset(nil, nil)

		defer recoverPanic(c)

		fn(c)
	}()
}

/*
 * Write the error to console or sentry when a goroutine of a task panics.
 */
func recoverPanic(c echo.Context) {

	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("%v", r)
		}

		// Run this function synchronously to release the `context` properly.
		sendErrorToSentry(c, err)
	}

	// Release the acquired context.
	global.EchoInstance.ReleaseContext(c)
}

// This function is only use in this package/file to avoid import cycle.
// For normal sentry usage, please refer to the `bean/internals/sentry` package.
func sendErrorToSentry(c echo.Context, err error) {

	isSentry := viper.GetBool("sentry.isSentry")
	sentryDSN := viper.GetString("sentry.dsn")

	if !isSentry || sentryDSN == "" {
		global.EchoInstance.Logger.Error(err)
		return
	}

	// IMPORTANT: Clone the current sentry hub from the echo context before it's gone.
	hub := sentryecho.GetHubFromContext(c)

	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}

	client, scope := hub.Client(), hub.Scope()

	event := sentry.NewEvent()
	event.Level = sentry.LevelError
	event.Exception = []sentry.Exception{{
		Value:      err.Error(),
		Type:       reflect.TypeOf(err).String(),
		Stacktrace: sentry.NewStacktrace(),
	}}

	client.CaptureEvent(event, &sentry.EventHint{RecoveredException: err}, scope)
}
