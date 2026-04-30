// Package observability wires OpenTelemetry into the app.
//
// The app exports OTLP over HTTP to a local collector. The collector handles
// fan-out, batching, and retries to backends. Configuration comes from the
// standard OTEL_* environment variables (OTEL_EXPORTER_OTLP_ENDPOINT,
// OTEL_SERVICE_NAME, etc.) so it can be tuned without code changes.
package observability

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Shutdown flushes pending telemetry and releases resources. Always defer it
// from main, with a context that has a sensible deadline so a stuck backend
// can't hang shutdown forever.
type Shutdown func(context.Context) error

// SetupLogs initializes the global OTel LoggerProvider and installs an slog
// handler that bridges slog calls to OTel log records.
//
// serviceName is recorded as the resource attribute service.name; pick a
// stable identifier (e.g. "ttc-web", "ttc-ssh"). Returns a Shutdown closure;
// callers must invoke it before the process exits.
func SetupLogs(ctx context.Context, serviceName string) (Shutdown, error) {
	exp, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("otlplog exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}

	processor := log.NewBatchProcessor(exp)
	provider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(processor),
	)
	global.SetLoggerProvider(provider)
	slog.SetDefault(otelslog.NewLogger(serviceName))

	return provider.Shutdown, nil
}
