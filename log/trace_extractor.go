package log

import (
	"context"

	"github.com/getsentry/sentry-go"
)

type TraceExtractor interface {
	Extract(ctx context.Context) Trace
}
type sentryExtractor struct{}

func NewSentryExtractor() *sentryExtractor {
	return &sentryExtractor{}
}

func (e *sentryExtractor) Extract(ctx context.Context) Trace {
	span := sentry.SpanFromContext(ctx)
	if span == nil {
		return Trace{}
	}

	return Trace{
		TraceID: span.TraceID.String(),
		SpanID:  span.SpanID.String(),
	}
}
