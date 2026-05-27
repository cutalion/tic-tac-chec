package config

import (
	"context"
	"strings"

	"github.com/sethvargo/go-envconfig"
)

type Server struct {
	Port           string   `env:"PORT, default=8080"`
	AllowedOrigins []string `env:"ALLOWED_ORIGINS"`
}

type Analytics struct {
	Enabled bool `env:"ANALYTICS_ENABLED, default=false"`
	PostHog *PostHog
}

type PostHog struct {
	Key  string `env:"POSTHOG_KEY"`
	Host string `env:"POSTHOG_HOST"`
}

type Database struct {
	DbPath string `env:"DB_PATH, default=tic-tac-chec.db"`
}

type Bots struct {
	OrtLibPath string `env:"ORT_LIB_PATH"`
}

// Logging configures slog output: stderr text (LOG_ENABLED) and/or OTLP (OTEL_ENABLED).
type Logging struct {
	OtelEnabled bool `env:"OTEL_ENABLED, default=false"`
	LogEnabled  bool `env:"LOG_ENABLED, default=true"`
}

type Config struct {
	Server    *Server
	Analytics *Analytics
	Database  *Database
	Bots      *Bots
	Logging   *Logging
}

func Load(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, err
	}
	// A set-but-empty PORT (e.g. `export PORT=`) overrides envDefault; treat as unset.
	if cfg.Server != nil && strings.TrimSpace(cfg.Server.Port) == "" {
		cfg.Server.Port = "8080"
	}
	return &cfg, nil
}
