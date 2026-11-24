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

package error

import (
	"fmt"
	"io"

	"github.com/retail-ai-inc/bean/v3/stacktrace"
)

// PanicError is returned by bean's recover middleware when a handler panics. It keeps the panic value and
// the stack trace of the panic site. The stack is exposed through `StackTrace()`, so Sentry shows where the
// panic happened (not where the error was reported).
type PanicError struct {
	Value any // the value passed to `panic`
	*stacktrace.Stack
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("[PANIC RECOVER] %v", e.Value)
}

// Unwrap returns the panic value if it is an error (e.g. `runtime.Error`), so `errors.Is` / `errors.As` work.
func (e *PanicError) Unwrap() error {
	err, _ := e.Value.(error)
	return err
}

// Format prints the stack trace of the panic site with `%+v`.
func (e *PanicError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, e.Error())
			if e.Stack != nil {
				e.Stack.Format(s, verb)
			}
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}
