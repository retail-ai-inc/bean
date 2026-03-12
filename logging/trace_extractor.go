package logging

import (
	"context"
	"github.com/retail-ai-inc/bean/v2/logging/types"
)

type TraceExtractor interface {
	Extract(ctx context.Context) types.Trace
}
