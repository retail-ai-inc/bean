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
	"github.com/retail-ai-inc/bean"
	"github.com/spf13/viper"
)

// TraceableContext contains a stack which holds the sentry span context information.
// Not thread safe.
type TraceableContext struct {
	stack []context.Context
	context.Context
}

func (c *TraceableContext) Push(ctx context.Context) {
	c.stack = append(c.stack, ctx)
	c.Context = ctx
}

func (c *TraceableContext) Pop() context.Context {
	if len(c.stack) <= 1 { // To sure the stack will always have a base context
		return c.Context
	}

	n := len(c.stack) - 1
	ctx := c.stack[n]        // Top element
	c.stack = c.stack[:n]    // Remove the top element in the stack
	c.Context = c.stack[n-1] // Set c.Context point to the new top element

	return ctx
}

// Start starts a span and return a finish() function to finish the corresponding span.
func Start(c context.Context, operation string, spanOpts ...sentry.SpanOption) func() {
	// if trace sample rate is 0.0 or 0 or sentry is off
	if viper.GetFloat64("sentry.tracesSampleRate") == 0 || !viper.GetBool("sentry.on") {
		return func() {}
	} else {
		functionName := "unknown function"
		pc, _, _, ok := runtime.Caller(1)
		if ok {
			functionName = runtime.FuncForPC(pc).Name()
		}

		var span *sentry.Span

		ctx, ok := c.(*TraceableContext)
		if ok {
			span = sentry.StartSpan(ctx.Context, operation, spanOpts...)
			ctx.Push(span.Context())
		} else {
			span = sentry.StartSpan(c, operation, spanOpts...)
			bean.SentryCaptureMessage(nil, functionName+"not using a traceable context")
		}

		span.Description = functionName

		finish := func() {
			if ctx != nil {
				ctx.Pop()
			}
			span.Finish()
		}
		return finish
	}
}

// NewTraceableContext return a traceable context which can hold different level of span information.
// This function Should be called in the upper layer only (handler, middleware...) and the lower layer
// reuse this context to create a hierarchy span tree.
func NewTraceableContext(ctx context.Context) *TraceableContext {
	stack := []context.Context{ctx}
	return &TraceableContext{
		stack:   stack,
		Context: ctx,
	}
}
