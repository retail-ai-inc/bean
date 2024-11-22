package trace_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v2/trace"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestStartSpan(t *testing.T) {

	tests := []struct {
		name     string
		setCfg   bool
		setupCtx func() context.Context
		newCtx   bool
	}{
		{
			name:   "start a new span with context that has a parent span and timeout",
			setCfg: true,
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				parentSpan := sentry.StartSpan(ctx, "parentOperation")
				defer parentSpan.Finish()
				return parentSpan.Context()
			},
			newCtx: true,
		},
		{
			name:   "start a new span with context that has no parent span but has a timeout",
			setCfg: true,
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				return ctx
			},
			newCtx: true,
		},
		{
			name:   "start a new span with context that has a timeout but sentry is disabled",
			setCfg: false,
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				return ctx
			},
			newCtx: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Arrange
			ctx := tt.setupCtx()
			originalDeadline, isSet := ctx.Deadline()
			require.True(t, isSet, "the setup context should have a timeout")
			type key struct{}
			var hogeKey key
			ctx = context.WithValue(ctx, hogeKey, "hoge")
			reset := setSentryConfig(tt.setCfg)
			defer reset()

			// Act
			newCtx, finish := trace.StartSpan(ctx, "test")
			defer finish()

			// Assert
			ptrCtx := reflect.ValueOf(ctx).Pointer()
			ptrNewCtx := reflect.ValueOf(newCtx).Pointer()
			if tt.newCtx {
				assert.NotEqual(t, ptrCtx, ptrNewCtx, "a new context should be created for the new span")
			} else {
				assert.Equal(t, ptrCtx, ptrNewCtx, "the context should remain unchanged when sentry is disabled")
			}
			newDeadline, isSet := newCtx.Deadline()
			assert.True(t, isSet, "the new context should have a timeout")
			assert.Equal(t, originalDeadline, newDeadline, "the deadline should be carried over")
			assert.Equal(t, ctx.Value(hogeKey), newCtx.Value(hogeKey), "the new context should carry over the original context values")
		})
	}
}

func setSentryConfig(enabled bool) func() {
	originalSampleRate := viper.GetFloat64("sentry.tracesSampleRate")
	originalSentryOn := viper.GetBool("sentry.on")

	if enabled {
		viper.Set("sentry.tracesSampleRate", 1.0)
		viper.Set("sentry.on", true)
	} else {
		viper.Set("sentry.tracesSampleRate", 0.0)
		viper.Set("sentry.on", false)
	}

	return func() {
		// Restore original config
		viper.Set("sentry.tracesSampleRate", originalSampleRate)
		viper.Set("sentry.on", originalSentryOn)
	}
}

func Test_Propagate_Sentry_Tracing_To_HTTP_Request(t *testing.T) {
	tests := []struct {
		name                string
		ctx                 context.Context
		setCfg              bool
		nonEmptySentryTrace bool
		baggage             string
	}{
		{
			name:                "propagate_sentry_tracing_info_to_HTTP_request",
			ctx:                 context.Background(),
			setCfg:              true,
			nonEmptySentryTrace: true,
			baggage:             "",
		},
		{
			name:                "no_sentry_tracing_information_to_propagate",
			ctx:                 context.Background(),
			setCfg:              false,
			nonEmptySentryTrace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Arrange
			reset := setSentryConfig(tt.setCfg)
			defer reset()
			ctx, finish := trace.StartSpan(tt.ctx, "test")
			defer finish()
			header := http.Header{}

			// Act
			got := trace.PropagateToHTTP(ctx, header)

			// Assert
			if tt.nonEmptySentryTrace {
				assert.NotEmpty(t, got.Get(sentry.SentryTraceHeader), "the sentry trace header should not be empty")
				assert.Equal(t, tt.baggage, got.Get(sentry.SentryBaggageHeader), "the sentry baggage header is not as expected")
			} else {
				assert.Empty(t, got.Get(sentry.SentryTraceHeader), "the sentry trace header should be empty")
				assert.Empty(t, got.Get(sentry.SentryBaggageHeader), "the sentry baggage header should be empty")
			}
		})
	}
}

func Test_Propagate_Sentry_Tracing_To_gRPC_Request(t *testing.T) {
	tests := []struct {
		name                string
		ctx                 context.Context
		setCfg              bool
		nonEmptySentryTrace bool
		baggage             string
	}{
		{
			name:                "propagate_sentry_tracing_info_to_gRPC_request",
			ctx:                 context.Background(),
			setCfg:              true,
			nonEmptySentryTrace: true,
			baggage:             "",
		},
		{
			name:                "no_sentry_tracing_information_to_propagate",
			ctx:                 context.Background(),
			setCfg:              false,
			nonEmptySentryTrace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Arrange
			reset := setSentryConfig(tt.setCfg)
			defer reset()
			ctx, finish := trace.StartSpan(tt.ctx, "test")
			defer finish()

			// Act
			got := trace.PropagateToGRPC(ctx)

			// Assert
			if tt.nonEmptySentryTrace {
				md, ok := metadata.FromOutgoingContext(got)
				require.True(t, ok, "the context should have metadata")
				if got := md.Get(sentry.SentryTraceHeader); len(got) != 1 || got[0] == "" {
					assert.Fail(t, "the sentry trace header should not be empty", fmt.Sprintf("got: %v", got))
				}
				if got := md.Get(sentry.SentryBaggageHeader); len(got) != 1 || got[0] != tt.baggage {
					assert.Fail(t, "the sentry baggage header is not as expected", fmt.Sprintf("got: %v", got))
				}
			} else {
				_, ok := metadata.FromOutgoingContext(got)
				require.False(t, ok, "the context should have metadata")
			}
		})
	}
}
