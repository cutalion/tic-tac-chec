package config

import (
	"context"
	"strings"

	"github.com/sethvargo/go-envconfig"
)

type Server struct {
	Port           string   `env:"PORT" envDefault:"8080"`
	AllowedOrigins []string `env:"ALLOWED_ORIGINS" envDefault:"*"`
}

type Analytics struct {
	Enabled bool `env:"ANALYTICS_ENABLED" envDefault:"false"`
	PostHog *PostHog
}

type PostHog struct {
	Key  string `env:"POSTHOG_KEY" envDefault:""`
	Host string `env:"POSTHOG_HOST" envDefault:""`
}

type Database struct {
	DbPath string `env:"DB_PATH" envDefault:"tic-tac-chec.db"`
}

type Bots struct {
	OrtLibPath string `env:"ORT_LIB_PATH" envDefault:""`
}

// Logging configures slog output: stderr text (LOG_ENABLED) and/or OTLP (OTEL_ENABLED).
type Logging struct {
	OtelEnabled bool `env:"OTEL_ENABLED" envDefault:"false"`
	LogEnabled  bool `env:"LOG_ENABLED" envDefault:"true"`
}

type Config struct {
	Server    *Server
	Analytics *Analytics
	Database  *Database
	Bots      *Bots
	Logging   *Logging
}

func NewConfig(ctx context.Context) (*Config, error) {
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
