package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"voxly/internal/bot"
	"voxly/internal/config"
	"voxly/internal/queue"
	"voxly/internal/storage"
	"voxly/pkg/cache"
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
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting voxly bot service")

	if *resetDB {
		logger.Info("Resetting database...")
		if err := storage.ResetMigrations(cfg.Database.DSN); err != nil {
			logger.Fatal("Failed to reset database", zap.Error(err))
			return
		}
		logger.Info("Database reset completed successfully")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := storage.NewPostgresStorage(cfg.Database.DSN)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
		return
	}
	defer db.Close()

	redisCache, err := cache.NewRedisCache(
		cfg.Cache.Redis.Addr,
		cfg.Cache.Redis.Password,
		cfg.Cache.Redis.DB,
		24*time.Hour,
	)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
		return
	}
	defer redisCache.Close()

	rabbitMQ, err := queue.NewRabbitMQ(cfg.Queue.RabbitMQ.URL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
		return
	}
	defer rabbitMQ.Close()

	botInstance, err := bot.NewBot(cfg, db, rabbitMQ, redisCache)
	if err != nil {
		logger.Fatal("Failed to initialize bot", zap.Error(err))
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("Starting Telegram bot")
		botInstance.Start()
	}()

	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	cancel()
	botInstance.Stop()

	logger.Info("Bot service shutdown complete")
}
