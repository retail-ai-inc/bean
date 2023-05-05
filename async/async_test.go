package async

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkExecute(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Execute(func() {
		})
	}
}

func BenchmarkExecuteWithContext(b *testing.B) {
	var request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"username":"testuser", "password":"testpass"}`)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExecuteWithContext(func(c context.Context) {

		}, request)
	}
}
