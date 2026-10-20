package async

import (
	"context"
	"errors"
	"testing"

	bctx "github.com/retail-ai-inc/bean/v2/context"
	"github.com/stretchr/testify/assert"
)

func TestAsyncFailureLogUsesIDKey(t *testing.T) {

	withID := bctx.SetRequestID(context.Background(), "req-1")

	tests := []struct {
		name string
		log  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "task error with request id",
			log:  taskFailedLog(withID, errors.New("boom")),
			want: map[string]interface{}{"message": "Async task failed.", "error": "boom", "id": "req-1"},
		},
		{
			name: "task error without request id",
			log:  taskFailedLog(context.Background(), errors.New("boom")),
			want: map[string]interface{}{"message": "Async task failed.", "error": "boom"},
		},
		{
			name: "panic with request id",
			log:  recoveredPanicLog(withID, "boom"),
			want: map[string]interface{}{"message": "Recovered panic from async task.", "cause": "boom", "id": "req-1"},
		},
		{
			name: "panic with nil context",
			//nolint:staticcheck // recoverPanic may receive a nil context from callers.
			log:  recoveredPanicLog(nil, "boom"),
			want: map[string]interface{}{"message": "Recovered panic from async task.", "cause": "boom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.log)
		})
	}
}
