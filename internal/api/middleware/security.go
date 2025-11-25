package middleware

import (
	"fmt"
	"net/http"
)

// SecurityHeadersConfig configures security headers middleware
type SecurityHeadersConfig struct {
	Enabled        bool
	HSTSEnabled    bool
	HSTSMaxAge     int
	FrameOptions   string
	ReferrerPolicy string
}

// DefaultSecurityHeadersConfig returns default security headers configuration
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		Enabled:        true,
		HSTSEnabled:    false, // Only enable in production with HTTPS
		HSTSMaxAge:     31536000,
		FrameOptions:   "DENY",
		ReferrerPolicy: "strict-origin-when-cross-origin",
	}
}

// SecurityHeaders returns middleware that sets security-related HTTP headers.
// These headers help protect against common web vulnerabilities.
func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Content Security Policy - restrict content sources
			// Note: This is a restrictive policy. Adjust based on your frontend needs.
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: https:; "+
					"font-src 'self'; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")

			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Clickjacking protection
			w.Header().Set("X-Frame-Options", cfg.FrameOptions)

			// XSS Filter (legacy but still useful for older browsers)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer Policy - control what referrer info is sent
			w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)

			// HTTP Strict Transport Security (only for HTTPS in production)
			if cfg.HSTSEnabled {
				w.Header().Set("Strict-Transport-Security",
					fmt.Sprintf("max-age=%d; includeSubDomains", cfg.HSTSMaxAge))
			}

			// Permissions Policy - disable browser features we don't need
			w.Header().Set("Permissions-Policy",
				"geolocation=(), microphone=(), camera=(), payment=(), usb=(), interest-cohort=()")

			// Cache control for API responses (no caching by default)
			// Note: This applies to all responses. Specific handlers can override if needed.
			if w.Header().Get("Cache-Control") == "" {
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			}

			next.ServeHTTP(w, r)
		})
	}
}
