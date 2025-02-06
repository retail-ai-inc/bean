package context

import (
	"context"
	"net/http"
)

type key struct{ string }

var (
	requestID   = key{"request_id"}
	httpRequest = key{"http_request"}
	// TODO: Add more keys here as needed.
)

func GetRequestID(ctx context.Context) (string, bool) {
	return getNotEmptyStr(ctx, requestID)
}

func SetRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestID, id)
}

func GetRequest(ctx context.Context) (*http.Request, bool) {
	return getNonNilPtr(ctx, httpRequest)
}

func SetRequest(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, httpRequest, req)
}

type ptr interface {
	*http.Request
	// TODO: Add more type constraints here as needed.
}

func getNotEmptyStr(ctx context.Context, k key) (string, bool) {
	str, ok := ctx.Value(k).(string)
	if !ok {
		return "", false
	}

	if str == "" {
		return "", false
	}

	return str, true
}

func getNonNilPtr[T ptr](ctx context.Context, k key) (T, bool) {
	v, ok := ctx.Value(k).(T)
	if !ok {
		return nil, false
	}

	// return false even when nil pointer is stored.
	if v == nil {
		return nil, false
	}

	return v, true
}
