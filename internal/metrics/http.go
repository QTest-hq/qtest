package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// HTTPRequestsTotal counts HTTP requests by method, endpoint pattern, and status code
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed, labeled by method, endpoint, and status code",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	// HTTPRequestDuration measures HTTP request latency by method and endpoint
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds, labeled by method and endpoint",
			Buckets:   latencyBuckets(),
		},
		[]string{"method", "endpoint"},
	)

	// HTTPRequestsInFlight tracks the number of requests currently being processed
	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Current number of HTTP requests being processed",
		},
	)

	// HTTPResponseSize measures HTTP response body sizes
	HTTPResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response body size in bytes",
			Buckets:   sizeBuckets(),
		},
		[]string{"method", "endpoint"},
	)
)

// RegisterHTTPMetrics registers all HTTP-related metrics with the registry
func RegisterHTTPMetrics() {
	Registry.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestsInFlight,
		HTTPResponseSize,
	)
}
