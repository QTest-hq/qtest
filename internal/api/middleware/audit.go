package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// AuditConfig configures audit logging behavior
type AuditConfig struct {
	// Enabled controls whether audit logging is active
	Enabled bool

	// SensitiveEndpoints are paths that should always be logged
	SensitiveEndpoints []string

	// ExcludedEndpoints are paths that should never be logged (e.g., health checks)
	ExcludedEndpoints []string

	// LogRequestBody controls whether request bodies are logged (careful with sensitive data!)
	LogRequestBody bool

	// LogResponseStatus controls whether response status is logged
	LogResponseStatus bool
}

// DefaultAuditConfig returns default audit configuration
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:           true,
		LogRequestBody:    false, // Disabled by default for security
		LogResponseStatus: true,
		SensitiveEndpoints: []string{
			"/auth/",
			"/api-keys",
			"/admin/",
			"/organizations/",
		},
		ExcludedEndpoints: []string{
			"/health",
			"/ready",
			"/metrics",
		},
	}
}

// AuditLogger is the interface for recording audit events
type AuditLogger interface {
	LogRequest(ctx context.Context, entry AuditEntry) error
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	Timestamp      time.Time
	RequestID      string
	Method         string
	Path           string
	RoutePattern   string
	StatusCode     int
	Duration       time.Duration
	IPAddress      string
	UserAgent      string
	UserID         string
	OrganizationID string
	IsSensitive    bool
}

// auditResponseWriter wraps http.ResponseWriter to capture the status code
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *auditResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

// Audit returns middleware that logs requests to sensitive endpoints.
// This is separate from standard HTTP logging and focuses on security-relevant events.
func Audit(cfg AuditConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check if this endpoint should be excluded
			for _, excluded := range cfg.ExcludedEndpoints {
				if strings.HasPrefix(r.URL.Path, excluded) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if this is a sensitive endpoint
			isSensitive := false
			for _, sensitive := range cfg.SensitiveEndpoints {
				if strings.Contains(r.URL.Path, sensitive) {
					isSensitive = true
					break
				}
			}

			// Start timing
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := &auditResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Only log sensitive endpoints or error responses
			shouldLog := isSensitive || wrapped.statusCode >= 400

			if shouldLog && cfg.LogResponseStatus {
				duration := time.Since(start)

				// Get route pattern
				routePattern := r.URL.Path
				if rctx := chi.RouteContext(r.Context()); rctx != nil {
					if pattern := rctx.RoutePattern(); pattern != "" {
						routePattern = pattern
					}
				}

				// Log the audit event
				logEvent := log.Info()
				if wrapped.statusCode >= 400 {
					logEvent = log.Warn()
				}
				if wrapped.statusCode >= 500 {
					logEvent = log.Error()
				}

				logEvent.
					Str("audit", "request").
					Str("request_id", chimiddleware.GetReqID(r.Context())).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("route", routePattern).
					Int("status", wrapped.statusCode).
					Dur("duration", duration).
					Str("ip", getClientIPForAudit(r)).
					Bool("sensitive", isSensitive).
					Msg("audit log")
			}
		})
	}
}

// getClientIPForAudit extracts client IP with privacy considerations
func getClientIPForAudit(r *http.Request) string {
	// Check X-Real-IP first (set by reverse proxy)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return xff
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
