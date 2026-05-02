// Package observability wires OpenTelemetry into the app.
//
// Logging is controlled by two booleans from [config.Logging]:
//   - LOG_ENABLED (default true): human-readable slog on stderr
//   - OTEL_ENABLED (default false): ship slog to OTLP (configure OTEL_EXPORTER_OTLP_* as needed)
//
// Both may be true (stderr + OTLP). Neither true uses a discard handler (quiet mode).
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"tic-tac-chec/internal/web/config"
)

// Shutdown flushes pending telemetry and releases resources. Always defer it
// from main, with a context that has a sensible deadline so a stuck backend
// can't hang shutdown forever.
type Shutdown func(context.Context) error

// SetupLogs configures the global slog logger from cfg (may be nil: treated as defaults).
func SetupLogs(ctx context.Context, serviceName string, cfg *config.Logging) (Shutdown, error) {
	otelOn := cfg != nil && cfg.OtelEnabled
	stderrOn := cfg == nil || cfg.LogEnabled // default true when cfg nil

	textHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})

	if !otelOn && stderrOn {
		slog.SetDefault(slog.New(textHandler))
		return func(context.Context) error { return nil }, nil
	}

	if !otelOn && !stderrOn {
		slog.SetDefault(slog.New(slog.DiscardHandler))
		return func(context.Context) error { return nil }, nil
	}

	// OTLP on
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

	otelHandler := otelslog.NewLogger(serviceName).Handler()
	if stderrOn {
		slog.SetDefault(slog.New(newFanout(
			otelHandler,
			textHandler,
		)))
	} else {
		slog.SetDefault(slog.New(otelHandler))
	}

	return provider.Shutdown, nil
}
