// Package middleware provides HTTP middleware for the QTest API.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QTest-hq/qtest/internal/metrics"
	"github.com/go-chi/chi/v5"
)

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default if WriteHeader is not called
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Flush implements http.Flusher for streaming responses
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Metrics returns a middleware that records HTTP metrics using Prometheus.
// It captures:
// - Request count by method, endpoint pattern, and status code
// - Request duration by method and endpoint pattern
// - Response size by method and endpoint pattern
// - Requests in flight (concurrent requests)
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Track requests in flight
			metrics.HTTPRequestsInFlight.Inc()
			defer metrics.HTTPRequestsInFlight.Dec()

			// Start timing
			start := time.Now()

			// Wrap response writer to capture status and size
			rw := newResponseWriter(w)

			// Process request
			next.ServeHTTP(rw, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Get the route pattern from chi context
			// This gives us the pattern like "/api/v1/jobs/{id}" instead of the actual path
			routePattern := getRoutePattern(r)

			// Record metrics
			statusCode := strconv.Itoa(rw.statusCode)

			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, statusCode).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, routePattern).Observe(duration)
			metrics.HTTPResponseSize.WithLabelValues(r.Method, routePattern).Observe(float64(rw.bytesWritten))
		})
	}
}

// getRoutePattern extracts the route pattern from chi's route context.
// Falls back to the URL path if no pattern is available.
func getRoutePattern(r *http.Request) string {
	// Get chi's route context
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return r.URL.Path
	}

	// Get the route pattern
	pattern := rctx.RoutePattern()
	if pattern == "" {
		return r.URL.Path
	}

	return pattern
}
