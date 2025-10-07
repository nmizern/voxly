package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"voxly/internal/config"
	"voxly/internal/queue"
	"voxly/internal/speechkit"
	"voxly/internal/storage"
	"voxly/internal/worker"
	"voxly/pkg/cache"
	"voxly/pkg/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v4"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	// Initialize logger
	debug := true
	if err := logger.Init(debug); err != nil {
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting voxly worker service")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
		return
	}

	// Connect to database
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Fatal("DATABASE_URL environment variable is required")
		return
	}

	db, err := storage.NewPostgresStorage(databaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
		return
	}
	defer db.Close()

	logger.Info("Database connection established")

	// Initialize S3 storage from config
	s3Storage, err := storage.NewS3Storage(
		cfg.S3.Endpoint,
		cfg.S3.AccessKey,
		cfg.S3.SecretKey,
		cfg.S3.Bucket,
	)
	if err != nil {
		logger.Fatal("Failed to initialize S3 storage", zap.Error(err))
		return
	}

	logger.Info("S3 storage initialized")

	// Initialize SpeechKit client
	speechkitClient := speechkit.NewClient(cfg.SpeechKit.APIKey, cfg.SpeechKit.FolderID)

	logger.Info("SpeechKit client initialized")

	// Initialize Telegram bot
	botSettings := tele.Settings{
		Token: cfg.Telegram.Token,
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	}

	bot, err := tele.NewBot(botSettings)
	if err != nil {
		logger.Fatal("Failed to create Telegram bot", zap.Error(err))
		return
	}

	logger.Info("Telegram bot initialized")

	// Initialize Redis cache
	redisCache, err := cache.NewRedisCache(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		24*time.Hour, // Default TTL 24 hours
	)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
		return
	}
	defer redisCache.Close()

	logger.Info("Redis cache connection established")

	// Connect to RabbitMQ
	rabbitMQ, err := queue.NewRabbitMQ(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
		return
	}
	defer rabbitMQ.Close()

	logger.Info("RabbitMQ connection established")

	// Create processor with cache
	processor := worker.NewProcessor(db, s3Storage, speechkitClient, bot, redisCache)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming messages
	go func() {
		logger.Info("Starting to consume messages from queue")
		if err := rabbitMQ.Consume(queue.QueueNameVoiceProcessing, processor.ProcessTask); err != nil {
			logger.Error("Failed to consume messages", zap.Error(err))
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	logger.Info("Worker service shutdown complete")
}
