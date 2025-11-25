package telemetry

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns an HTTP middleware that adds OpenTelemetry tracing.
// It wraps otelhttp and adds additional attributes like request ID and route pattern.
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Wrap with otelhttp handler
		otelHandler := otelhttp.NewHandler(next, serviceName,
			otelhttp.WithSpanNameFormatter(spanNameFormatter),
		)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add request ID to span if available
			span := trace.SpanFromContext(r.Context())
			if span.IsRecording() {
				// Add Chi request ID
				if reqID := middleware.GetReqID(r.Context()); reqID != "" {
					span.SetAttributes(attribute.String("http.request_id", reqID))
				}

				// Add route pattern after routing
				if rctx := chi.RouteContext(r.Context()); rctx != nil {
					if pattern := rctx.RoutePattern(); pattern != "" {
						span.SetAttributes(attribute.String("http.route", pattern))
					}
				}
			}

			otelHandler.ServeHTTP(w, r)
		})
	}
}

// spanNameFormatter creates span names in the format "METHOD /path"
func spanNameFormatter(_ string, r *http.Request) string {
	// Try to get the route pattern from chi
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return r.Method + " " + pattern
		}
	}
	return r.Method + " " + r.URL.Path
}
