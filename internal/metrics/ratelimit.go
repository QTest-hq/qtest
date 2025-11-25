package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// RateLimitHits counts rate limit checks that succeeded (request allowed)
	RateLimitHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "ratelimit",
			Name:      "hits_total",
			Help:      "Total number of rate limit checks that allowed the request, labeled by key type and window",
		},
		[]string{"key_type", "window"},
	)

	// RateLimitRejections counts rate limit rejections
	RateLimitRejections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "ratelimit",
			Name:      "rejections_total",
			Help:      "Total number of rate limit rejections, labeled by key type and window",
		},
		[]string{"key_type", "window"},
	)

	// RateLimitCurrentUsage tracks current usage against rate limits
	RateLimitCurrentUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "ratelimit",
			Name:      "current_usage",
			Help:      "Current usage count for rate limited resources, labeled by key type",
		},
		[]string{"key_type"},
	)

	// RateLimitBucketSize tracks the configured bucket size for rate limits
	RateLimitBucketSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "ratelimit",
			Name:      "bucket_size",
			Help:      "Configured bucket size for rate limits, labeled by key type",
		},
		[]string{"key_type"},
	)
)

// RegisterRateLimitMetrics registers all rate limiting metrics with the registry
func RegisterRateLimitMetrics() {
	Registry.MustRegister(
		RateLimitHits,
		RateLimitRejections,
		RateLimitCurrentUsage,
		RateLimitBucketSize,
	)
}
