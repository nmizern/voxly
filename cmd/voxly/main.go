package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"voxly/internal/app"
	"voxly/internal/config"
	"voxly/internal/storage"
	"voxly/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	resetDB := flag.Bool("reset-db", false, "Drop all tables and re-run migrations")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.Observability.LogLevel, cfg.Observability.LogFormat); err != nil {
		panic("failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	if *resetDB {
		if cfg.Database.Driver != "postgres" {
			logger.Fatal("reset-db requires the postgres database driver")
		}
		if err := storage.ResetMigrations(cfg.Database.DSN); err != nil {
			logger.Fatal("Failed to reset database", zap.Error(err))
		}
		logger.Info("Database reset completed")
		return
	}

	a, err := app.New(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize app", zap.Error(err))
	}
	defer a.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("Voxly starting",
		zap.String("mode", string(cfg.Mode)),
		zap.String("role", cfg.Role),
		zap.String("provider", string(cfg.STT.Provider)))

	if err := a.Run(ctx); err != nil {
		logger.Error("Run error", zap.Error(err))
	}
	logger.Info("Voxly stopped")
}
