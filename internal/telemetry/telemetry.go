// Package telemetry provides OpenTelemetry distributed tracing for QTest services.
package telemetry

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds telemetry configuration
type Config struct {
	Enabled      bool
	OTLPEndpoint string
	SamplingRate float64
	ServiceName  string
	Environment  string
}

// DefaultConfig returns the default telemetry configuration
func DefaultConfig() Config {
	return Config{
		Enabled:      false,
		OTLPEndpoint: "localhost:4317",
		SamplingRate: 1.0,
		ServiceName:  "qtest",
		Environment:  "development",
	}
}

// Provider wraps the OpenTelemetry tracer provider
type Provider struct {
	tp     *sdktrace.TracerProvider
	tracer trace.Tracer
	config Config
}

// New creates a new telemetry provider with the given configuration.
// Returns nil if telemetry is disabled.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		log.Info().Msg("telemetry disabled")
		return nil, nil
	}

	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // Use insecure for local development
		otlptracegrpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create sampler based on config
	var sampler sdktrace.Sampler
	if cfg.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SamplingRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRate)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global provider
	otel.SetTracerProvider(tp)

	// Set W3C Trace Context propagator for distributed tracing
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info().
		Str("endpoint", cfg.OTLPEndpoint).
		Str("service", cfg.ServiceName).
		Float64("sampling_rate", cfg.SamplingRate).
		Msg("telemetry initialized")

	return &Provider{
		tp:     tp,
		tracer: tp.Tracer(cfg.ServiceName),
		config: cfg,
	}, nil
}

// Tracer returns the configured tracer
func (p *Provider) Tracer() trace.Tracer {
	if p == nil {
		return otel.Tracer("qtest")
	}
	return p.tracer
}

// TracerProvider returns the underlying tracer provider
func (p *Provider) TracerProvider() *sdktrace.TracerProvider {
	if p == nil {
		return nil
	}
	return p.tp
}

// Shutdown gracefully shuts down the telemetry provider
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.tp == nil {
		return nil
	}

	log.Info().Msg("shutting down telemetry provider")
	return p.tp.Shutdown(ctx)
}

// StartSpan starts a new span with the given name
func (p *Provider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.Tracer().Start(ctx, name, opts...)
}

// GetTraceID extracts the trace ID from the context as a string
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	sc := span.SpanContext()
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// GetSpanID extracts the span ID from the context as a string
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	sc := span.SpanContext()
	if !sc.HasSpanID() {
		return ""
	}
	return sc.SpanID().String()
}
