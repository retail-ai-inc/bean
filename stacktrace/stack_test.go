package stacktrace

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
)

var errBase = errors.New("base")

func failingCall() error { return WithStack(fmt.Errorf("wrapped: %w", errBase)) }

func TestWithStack(t *testing.T) {
	if WithStack(nil) != nil {
		t.Fatal("WithStack(nil) must be nil")
	}

	err := failingCall()
	if !errors.Is(err, errBase) {
		t.Fatal("errors.Is must see the wrapped error")
	}
	if got := fmt.Sprintf("%v", err); got != "wrapped: base" {
		t.Fatalf("%%v = %q", got)
	}
	if got := fmt.Sprintf("%+v", err); !strings.HasPrefix(got, "wrapped: base\n") || !strings.Contains(got, "failingCall") {
		t.Fatalf("%%+v must print the error and the stack of the caller, got %q", got)
	}

	// Sentry reads the stack through StackTrace().
	st := sentry.ExtractStacktrace(err)
	if st == nil || len(st.Frames) == 0 {
		t.Fatal("sentry must extract the stack")
	}
	found := false
	for _, f := range st.Frames {
		if strings.Contains(f.Function, "failingCall") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the sentry stack must contain the caller, got %+v", st.Frames)
	}
}
