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
	"runtime"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/viper"
)

// StartSpan starts a span and returns context containing the span and a function to finish the corresponding span.
// Make sure to call the returned function to finish the span.
func StartSpan(c context.Context, operation string, spanOpts ...sentry.SpanOption) (context.Context, func()) {
	// If trace sample rate is 0.0 or 0 or Sentry is off, use the provided context as-is.
	if viper.GetFloat64("sentry.tracesSampleRate") == 0 || !viper.GetBool("sentry.on") {
		return c, func() {}
	}

	if len(spanOpts) == 0 {
		// Add defalut options if none provided.
		functionName := "unknown function"
		if pc, _, _, ok := runtime.Caller(1); ok {
			functionName = runtime.FuncForPC(pc).Name()
		}
		spanOpts = append(spanOpts, sentry.WithDescription(functionName))
	}

	span := sentry.StartSpan(c, operation, spanOpts...)
	newCtx := span.Context()

	return newCtx, func() {
		span.Finish()
	}
}
