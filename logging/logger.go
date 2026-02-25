package logging

import (
	"bytes"
	"context"
)

type Logger interface {
	Log(entry Entry)
	AppendTrace(ctx context.Context, buf *bytes.Buffer)
}
