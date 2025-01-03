// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package trace

import (
	"context"
	"net/http"
	"runtime"

	sentryecho "github.com/getsentry/sentry-go/echo"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	berror "github.com/retail-ai-inc/bean/v2/error"
	"github.com/retail-ai-inc/bean/v2/internal/validator"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
)

// StartSpanWithEcho starts a span and returns context containing the span and a function to finish the corresponding span.
// It also carries over the sentry hub from the echo context to child context.
// Make sure to call the returned function to finish the span.
func StartSpanWithEcho(c echo.Context, operation string, spanOpts ...sentry.SpanOption) (context.Context, func()) {

	ctx := c.Request().Context()

	if sentry.GetHubFromContext(ctx) == nil {
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			ctx = sentry.SetHubOnContext(ctx, hub)
		}
	}

	return startSpan(ctx, operation, 1, spanOpts...)
}

// StartSpan starts a span and returns context containing the span and a function to finish the corresponding span.
// Make sure to call the returned function to finish the span.
func StartSpan(c context.Context, operation string, spanOpts ...sentry.SpanOption) (context.Context, func()) {
	return startSpan(c, operation, 1, spanOpts...)
}

// startSpan starts a span and returns context containing the span and a function to finish the corresponding span.
func startSpan(c context.Context, operation string, skip int, spanOpts ...sentry.SpanOption) (context.Context, func()) {
	// If trace sample rate is 0.0 or 0 or Sentry is off, use the provided context as-is.
	if viper.GetFloat64("sentry.tracesSampleRate") == 0 || !viper.GetBool("sentry.on") {
		return c, func() {}
	}

	if len(spanOpts) == 0 {
		// Add default options if none provided.
		spanOpts = append(spanOpts, defaultDescription(skip+1))
	}

	span := sentry.StartSpan(c, operation, spanOpts...)
	newCtx := span.Context()

	return newCtx, func() {
		span.Finish()
	}
}

func defaultDescription(skip int) sentry.SpanOption {

	functionName := "unknown function"
	if pc, _, _, ok := runtime.Caller(skip + 1); ok {
		functionName = runtime.FuncForPC(pc).Name()
	}

	return sentry.WithDescription(functionName)
}

// PropagateToHTTP propagates the Sentry tracing information to the outgoing HTTP/1.X request header.
// Refers to the following link for more information.
// https://docs.sentry.io/platforms/go/tracing/trace-propagation/custom-instrumentation/#step-2-inject-tracing-information-to-outgoing-requests
func PropagateToHTTP(ctx context.Context, header http.Header) http.Header {

	sentryTrace, baggage := extractTracing(ctx)
	if sentryTrace == "" {
		return header
	}

	header.Add(sentry.SentryTraceHeader, sentryTrace)
	header.Add(sentry.SentryBaggageHeader, baggage)

	return header
}

// PropagateToGRPC propagates the Sentry tracing information to the outgoing gRPC request metadata.
// Refers to the following link for more information.
// https://docs.sentry.io/platforms/go/tracing/trace-propagation/custom-instrumentation/#step-2-inject-tracing-information-to-outgoing-requests
func PropagateToGRPC(ctx context.Context) context.Context {

	sentryTrace, baggage := extractTracing(ctx)
	if sentryTrace == "" {
		return ctx
	}

	md := metadata.Pairs(
		sentry.SentryTraceHeader, sentryTrace,
		sentry.SentryBaggageHeader, baggage,
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	return ctx
}

func extractTracing(ctx context.Context) (sentryTrace, baggage string) {

	span := sentry.SpanFromContext(ctx)
	if span == nil {
		return "", ""
	}

	return span.ToSentryTrace(), span.ToBaggage()
}

// Modify event through beforeSend function.
func DefaultBeforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	// Example: enriching the event by adding aditional data.
	switch err := hint.OriginalException.(type) {
	case *validator.ValidationError:
		return event
	case *berror.APIError:
		if err.Ignorable {
			return nil
		}
		event.Contexts["Error"] = map[string]interface{}{
			"HTTPStatusCode": err.HTTPStatusCode,
			"GlobalErrCode":  err.GlobalErrCode,
			"Message":        err.Error(),
		}
		return event
	case *echo.HTTPError:
		return event
	default:
		return event
	}
}

// Modify breadcrumbs through beforeBreadcrumb function.
func DefaultBeforeBreadcrumb(breadcrumb *sentry.Breadcrumb, hint *sentry.BreadcrumbHint) *sentry.Breadcrumb {
	// Example: discard the breadcrumb by return nil.
	// if breadcrumb.Category == "example" {
	// 	return nil
	// }
	return breadcrumb
}
