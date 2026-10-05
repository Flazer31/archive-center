package vector

import (
	"context"
	"strings"
)

type queryResponseObserverKey struct{}

// WithQueryResponseObserver observes the already-read HTTP body length, without
// decoding or retaining another copy. A callback must be safe for its callers.
func WithQueryResponseObserver(ctx context.Context, observe func(bytes, status int)) context.Context {
	return context.WithValue(ctx, queryResponseObserverKey{}, observe)
}

func observeQueryResponse(ctx context.Context, path string, bytes, status int) {
	if !strings.HasSuffix(path, "/query") {
		return
	}
	if observe, ok := ctx.Value(queryResponseObserverKey{}).(func(int, int)); ok && observe != nil {
		observe(bytes, status)
	}
}
