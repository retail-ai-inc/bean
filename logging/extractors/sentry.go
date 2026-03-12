package extractors

import (
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v2/logging/types"
)

type SentryExtractor struct{}

func NewSentryExtractor() *SentryExtractor {
	return &SentryExtractor{}
}

func (e *SentryExtractor) Extract(ctx context.Context) types.Trace {
	span := sentry.SpanFromContext(ctx)
	if span == nil {
		return types.Trace{}
	}

	return types.Trace{
		TraceID: span.TraceID.String(),
		SpanID:  span.SpanID.String(),
	}
}
