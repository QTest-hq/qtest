// Package metrics provides Prometheus instrumentation for QTest services.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Namespace is the prefix for all QTest metrics
	Namespace = "qtest"
)

// Registry is the custom Prometheus registry for QTest metrics.
// Using a custom registry avoids conflicts with default collectors and gives us
// full control over what metrics are exposed.
var Registry = prometheus.NewRegistry()

func init() {
	// Register standard Go runtime and process collectors
	Registry.MustRegister(collectors.NewGoCollector())
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Register all QTest metric collectors
	RegisterHTTPMetrics()
	RegisterDatabaseMetrics()
	RegisterNATSMetrics()
	RegisterJobMetrics()
	RegisterLLMMetrics()
	RegisterRateLimitMetrics()
}

// Handler returns an HTTP handler for the /metrics endpoint using our custom registry.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          Registry,
	})
}

// MustRegister registers the provided collectors with the custom registry.
// Panics if registration fails.
func MustRegister(cs ...prometheus.Collector) {
	Registry.MustRegister(cs...)
}

// buckets returns common histogram buckets for latency measurements
func latencyBuckets() []float64 {
	return []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
}

// sizeBuckets returns histogram buckets for response size measurements (bytes)
func sizeBuckets() []float64 {
	return []float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000}
}

// jobDurationBuckets returns histogram buckets for job duration measurements (seconds)
func jobDurationBuckets() []float64 {
	return []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800}
}
