package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// MapCarrier implements propagation.TextMapCarrier for map[string]string.
// This allows trace context to be propagated through NATS messages.
type MapCarrier map[string]string

// Get returns the value associated with the passed key.
func (c MapCarrier) Get(key string) string {
	return c[key]
}

// Set stores the key-value pair.
func (c MapCarrier) Set(key, value string) {
	c[key] = value
}

// Keys returns a slice of all keys in the carrier.
func (c MapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTraceContext extracts trace context from the context and returns it as a map.
// This map can be serialized and sent with NATS messages.
func InjectTraceContext(ctx context.Context) map[string]string {
	carrier := make(MapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier
}

// ExtractTraceContext creates a context with trace information from the carrier map.
// Use this when receiving NATS messages to continue the trace.
func ExtractTraceContext(ctx context.Context, carrier map[string]string) context.Context {
	if carrier == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, MapCarrier(carrier))
}

// StartNATSPublishSpan starts a new span for publishing a NATS message.
func StartNATSPublishSpan(ctx context.Context, subject string) (context.Context, trace.Span) {
	tracer := otel.Tracer("qtest-nats")
	ctx, span := tracer.Start(ctx, "NATS publish "+subject,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination", subject),
			attribute.String("messaging.operation", "publish"),
		),
	)
	return ctx, span
}

// StartNATSConsumeSpan starts a new span for consuming a NATS message.
// It should be called after ExtractTraceContext to properly link the trace.
func StartNATSConsumeSpan(ctx context.Context, subject, consumerName string) (context.Context, trace.Span) {
	tracer := otel.Tracer("qtest-nats")
	ctx, span := tracer.Start(ctx, "NATS consume "+subject,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination", subject),
			attribute.String("messaging.consumer.id", consumerName),
			attribute.String("messaging.operation", "receive"),
		),
	)
	return ctx, span
}
