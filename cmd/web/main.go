package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tic-tac-chec/internal/observability"
	"tic-tac-chec/internal/web/app"
	"tic-tac-chec/internal/web/config"
	store "tic-tac-chec/internal/web/persistence/sqlite"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := config.NewConfig(ctx)

	if err != nil {
		log.Fatal(err)
	}

	if err := run(ctx, *config); err != nil {
		log.Fatal(err)
	}

	log.Println("shutdown complete")
}

func run(ctx context.Context, cfg config.Config) error {
	shutdown, err := observability.SetupLogs(ctx, "web", cfg.Logging)
	if err != nil {
		return fmt.Errorf("setup logs: %w", err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(sctx)
	}()

	db, err := store.NewStore(cfg.Database.DbPath)
	if err != nil {
		return fmt.Errorf("store init: %w", err)
	}
	defer db.Close()

	a := app.NewApp(ctx, db, cfg)
	if err := a.Run(ctx); err != nil {
		return fmt.Errorf("app run: %w", err)
	}

	return nil
}
