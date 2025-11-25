package request

import (
	"context"
	"time"
)

// Single typed context key
type contextKey string

const requestContextKey contextKey = "request"

// RequestContext holds request-scoped data for tracing and monitoring
type RequestContext struct {
	RequestID string    // Unique ID for request tracing
	StartTime time.Time // For latency tracking
}

// SetContext stores request context
func SetContext(ctx context.Context, reqCtx *RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey, reqCtx)
}

// GetContext retrieves request context with safe fallback
func GetContext(ctx context.Context) *RequestContext {
	if reqCtx, ok := ctx.Value(requestContextKey).(*RequestContext); ok {
		return reqCtx
	}
	return &RequestContext{} // Safe fallback
}

// GetRequestID is a convenience function
func GetRequestID(ctx context.Context) string {
	return GetContext(ctx).RequestID
}
