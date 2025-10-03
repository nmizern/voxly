package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"voxly/internal/bot"
	"voxly/internal/config"
	"voxly/internal/queue"
	"voxly/internal/storage"
	"voxly/pkg/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load .env file first
	_ = godotenv.Load()

	// Parse command line flags
	resetDB := flag.Bool("reset-db", false, "Reset database by dropping all tables and re-running migrations")
	flag.Parse()

	// Initialize the logger first
	debug := true // or false, depending on your needs
	if err := logger.Init(debug); err != nil {
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting voxly bot service")

	// Get database URL
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Fatal("DATABASE_URL environment variable is required")
		return
	}

	// Reset database if flag is provided
	if *resetDB {
		logger.Info("Resetting database...")
		if err := storage.ResetMigrations(databaseURL); err != nil {
			logger.Fatal("Failed to reset database", zap.Error(err))
			return
		}
		logger.Info("Database reset completed successfully")
		return
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
		return
	}

	// Initialize database connection
	db, err := storage.NewPostgresStorage(databaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
		return
	}
	defer db.Close()

	logger.Info("Database connection established")

	// Connect to RabbitMQ
	rabbitMQ, err := queue.NewRabbitMQ(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
		return
	}
	defer rabbitMQ.Close()

	logger.Info("RabbitMQ connection established")

	// Initialize bot with database and queue
	botInstance, err := bot.NewBot(cfg, db, rabbitMQ)
	if err != nil {
		logger.Fatal("Failed to initialize bot", zap.Error(err))
		return
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start bot in a goroutine
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

	// Graceful shutdown
	cancel()
	botInstance.Stop()

	logger.Info("Bot service shutdown complete")
}
