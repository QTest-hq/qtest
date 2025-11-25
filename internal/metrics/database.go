package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// DBConnectionsOpen tracks the total number of open database connections
	DBConnectionsOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "db",
			Name:      "connections_open",
			Help:      "Total number of open database connections",
		},
	)

	// DBConnectionsInUse tracks the number of database connections currently in use
	DBConnectionsInUse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "db",
			Name:      "connections_in_use",
			Help:      "Number of database connections currently in use",
		},
	)

	// DBConnectionsIdle tracks the number of idle database connections
	DBConnectionsIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "db",
			Name:      "connections_idle",
			Help:      "Number of idle database connections in the pool",
		},
	)

	// DBConnectionsMaxOpen tracks the maximum allowed open connections
	DBConnectionsMaxOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "db",
			Name:      "connections_max_open",
			Help:      "Maximum number of open connections allowed to the database",
		},
	)

	// DBQueryDuration measures database query latency by operation type
	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: "db",
			Name:      "query_duration_seconds",
			Help:      "Database query duration in seconds, labeled by operation type",
			Buckets:   latencyBuckets(),
		},
		[]string{"operation"},
	)

	// DBQueryErrorsTotal counts database query errors by operation type
	DBQueryErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "db",
			Name:      "query_errors_total",
			Help:      "Total number of database query errors, labeled by operation type",
		},
		[]string{"operation"},
	)

	// DBConnectionWaitDuration measures time spent waiting for a connection
	DBConnectionWaitDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: "db",
			Name:      "connection_wait_duration_seconds",
			Help:      "Time spent waiting for a database connection from the pool",
			Buckets:   latencyBuckets(),
		},
	)
)

// RegisterDatabaseMetrics registers all database-related metrics with the registry
func RegisterDatabaseMetrics() {
	Registry.MustRegister(
		DBConnectionsOpen,
		DBConnectionsInUse,
		DBConnectionsIdle,
		DBConnectionsMaxOpen,
		DBQueryDuration,
		DBQueryErrorsTotal,
		DBConnectionWaitDuration,
	)
}
