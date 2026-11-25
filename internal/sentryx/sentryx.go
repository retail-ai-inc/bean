// Package sentryx holds the Sentry helpers shared by the async, sync and trace packages.
package sentryx

import (
	"context"
	"fmt"
	"path"
	"runtime"

	"github.com/getsentry/sentry-go"
)

// CapturePanic reports the value v of a recovered panic of a goroutine to Sentry, with a clone of the hub of ctx
// (the current hub if ctx has none) tagged `goroutine: true`.
func CapturePanic(ctx context.Context, v any) {
	var hub *sentry.Hub
	if ctx != nil {
		hub = sentry.GetHubFromContext(ctx)
	}
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	clone := hub.Clone()
	clone.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("goroutine", "true")
	})
	clone.Recover(v)
}

// TraceHeaders returns the `sentry-trace` and `baggage` values of the span of ctx, empty if there is none.
func TraceHeaders(ctx context.Context) (sentryTrace, baggage string) {
	span := sentry.SpanFromContext(ctx)
	if span == nil {
		return "", ""
	}
	return span.ToSentryTrace(), span.ToBaggage()
}

// CallerDescription describes the caller skip frames above the caller of CallerDescription (0: the function calling
// CallerDescription) as `file.go:line` and its function name, for span descriptions.
func CallerDescription(skip int) string {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown function"
	}
	name := "unknown function"
	if fn := runtime.FuncForPC(pc); fn != nil {
		name = fn.Name()
	}
	return fmt.Sprintf("%s:%d\n\t\r %s\n", path.Base(file), line, name)
}
