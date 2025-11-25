package middleware

import (
	"net/http"
)

const (
	// DefaultBodyLimit is the default maximum request body size (1MB)
	DefaultBodyLimit int64 = 1 * 1024 * 1024

	// LargeBodyLimit is for endpoints that accept larger payloads (50MB)
	LargeBodyLimit int64 = 50 * 1024 * 1024
)

// BodyLimit returns a middleware that limits the size of the request body.
// Requests with bodies larger than the limit will receive a 413 Request Entity Too Large response.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Wrap the body reader with a limited reader
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next.ServeHTTP(w, r)
		})
	}
}

// DefaultBodyLimitMiddleware returns a middleware with the default body limit
func DefaultBodyLimitMiddleware() func(http.Handler) http.Handler {
	return BodyLimit(DefaultBodyLimit)
}

// LargeBodyLimitMiddleware returns a middleware for endpoints accepting larger payloads
func LargeBodyLimitMiddleware() func(http.Handler) http.Handler {
	return BodyLimit(LargeBodyLimit)
}
