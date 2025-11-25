package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// NATSMessagesPublished counts messages published to NATS by subject
	NATSMessagesPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "nats",
			Name:      "messages_published_total",
			Help:      "Total number of messages published to NATS, labeled by subject",
		},
		[]string{"subject"},
	)

	// NATSMessagesReceived counts messages received from NATS by subject
	NATSMessagesReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "nats",
			Name:      "messages_received_total",
			Help:      "Total number of messages received from NATS, labeled by subject",
		},
		[]string{"subject"},
	)

	// NATSMessageProcessingDuration measures message processing time by subject
	NATSMessageProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: "nats",
			Name:      "message_processing_duration_seconds",
			Help:      "Time spent processing NATS messages, labeled by subject",
			Buckets:   jobDurationBuckets(),
		},
		[]string{"subject"},
	)

	// NATSConnectionState indicates the current connection state (0=disconnected, 1=connected)
	NATSConnectionState = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "nats",
			Name:      "connection_state",
			Help:      "Current NATS connection state (0=disconnected, 1=connected, 2=reconnecting)",
		},
	)

	// NATSPendingMessages tracks the number of pending messages by consumer
	NATSPendingMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: "nats",
			Name:      "pending_messages",
			Help:      "Number of pending messages in NATS consumer queue, labeled by consumer",
		},
		[]string{"consumer"},
	)

	// NATSPublishErrors counts publish errors by subject
	NATSPublishErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "nats",
			Name:      "publish_errors_total",
			Help:      "Total number of NATS publish errors, labeled by subject",
		},
		[]string{"subject"},
	)

	// NATSReconnects counts the number of reconnection attempts
	NATSReconnects = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: "nats",
			Name:      "reconnects_total",
			Help:      "Total number of NATS reconnection attempts",
		},
	)
)

// RegisterNATSMetrics registers all NATS-related metrics with the registry
func RegisterNATSMetrics() {
	Registry.MustRegister(
		NATSMessagesPublished,
		NATSMessagesReceived,
		NATSMessageProcessingDuration,
		NATSConnectionState,
		NATSPendingMessages,
		NATSPublishErrors,
		NATSReconnects,
	)
}
