package telemetry

import (
	"context"

	"github.com/rs/zerolog"
)

// LoggerWithTrace returns a logger with trace_id and span_id fields added
// from the current span in the context.
func LoggerWithTrace(ctx context.Context, logger zerolog.Logger) zerolog.Logger {
	traceID := GetTraceID(ctx)
	spanID := GetSpanID(ctx)

	if traceID != "" || spanID != "" {
		logCtx := logger.With()
		if traceID != "" {
			logCtx = logCtx.Str("trace_id", traceID)
		}
		if spanID != "" {
			logCtx = logCtx.Str("span_id", spanID)
		}
		return logCtx.Logger()
	}

	return logger
}

// LoggerFromContext returns a logger with trace context from the given context.
// This is a convenience function that creates a sub-logger with trace fields.
func LoggerFromContext(ctx context.Context, baseLogger zerolog.Logger) zerolog.Logger {
	return LoggerWithTrace(ctx, baseLogger)
}
