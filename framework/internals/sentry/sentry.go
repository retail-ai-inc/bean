/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package sentry

import (
	"errors"
	"reflect"

	"bean/framework/internals/global"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

const maxErrorDepth = 10

// PushData posting error/info in to sentry or console asynchronously.
func PushData(c echo.Context, data interface{}, event *sentry.Event, isAsync bool) {

	isSentry := viper.GetBool("sentry.isSentry")
	sentryDSN := viper.GetString("sentry.dsn")

	// Check sentry is active or not and if active then priorartize sentry over console.log or stdout.
	if !isSentry || sentryDSN == "" {

		if data, ok := data.(error); ok {
			global.EchoInstance.Logger.Error(data)
		} else {
			global.EchoInstance.Logger.Info(data)
		}

		return
	}

	// IMPORTANT: Clone the current sentry hub from the echo context before it's gone.
	hub := sentryecho.GetHubFromContext(c)
	if hub != nil {
		hub = hub.Clone()
	}

	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}

	if isAsync {
		go sendEventToSentry(hub, event, data)
	} else {
		sendEventToSentry(hub, event, data)
	}
}

func sendEventToSentry(hub *sentry.Hub, event *sentry.Event, data interface{}) {

	client, scope := hub.Client(), hub.Scope()

	if exception, ok := data.(error); ok {

		if event == nil {
			event = EventFromException(exception)
		}
		client.CaptureEvent(event, &sentry.EventHint{RecoveredException: exception}, scope)

	} else {

		if event == nil {
			event = EventFromRawData(data, sentry.LevelError)
		}
		client.CaptureEvent(event, &sentry.EventHint{Data: data}, scope)
	}
}

// EventFromException creates an sentry event from error.
func EventFromException(exception error) *sentry.Event {

	err := exception
	if err == nil {
		err = errors.New("called with nil error")
	}

	event := sentry.NewEvent()
	event.Level = sentry.LevelError

	for i := 0; i < maxErrorDepth && err != nil; i++ {
		event.Exception = append(event.Exception, sentry.Exception{
			Value:      err.Error(),
			Type:       reflect.TypeOf(err).String(),
			Stacktrace: sentry.ExtractStacktrace(err),
		})
		switch previous := err.(type) {
		case interface{ Unwrap() error }:
			err = previous.Unwrap()
		case interface{ Cause() error }:
			err = previous.Cause()
		default:
			err = nil
		}
	}

	// Add a trace of the current stack to the most recent error in a chain if
	// it doesn't have a stack trace yet.
	// We only add to the most recent error to avoid duplication and because the
	// current stack is most likely unrelated to errors deeper in the chain.
	if event.Exception[0].Stacktrace == nil {
		event.Exception[0].Stacktrace = sentry.NewStacktrace()
	}

	// event.Exception should be sorted such that the most recent error is last.
	reverse(event.Exception)

	return event
}

// EventFromRawData creates an sentry event.
func EventFromRawData(data interface{}, level sentry.Level) *sentry.Event {

	if data == nil {
		err := errors.New("called with nil data")
		return EventFromException(err)
	}

	event := sentry.NewEvent()
	event.Level = level
	event.Extra["raw"] = data

	event.Threads = []sentry.Thread{{
		Stacktrace: NewStacktrace(),
		Crashed:    false,
		Current:    true,
	}}

	return event
}

// Do not change: function copyied from sentry library
// reverse reverses the slice a in place.
func reverse(a []sentry.Exception) {
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}
}

// NewStacktrace returns new sentry stacktrace
func NewStacktrace() *sentry.Stacktrace {
	return sentry.NewStacktrace()
}
