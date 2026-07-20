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

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v4"
)

func main() {
	if err := logger.Init(true); err != nil {
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting voxly worker service")

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
		return
	}

	db, err := storage.NewPostgresStorage(cfg.Database.DSN)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
		return
	}
	defer db.Close()

	s3Storage, err := storage.NewS3Storage(
		cfg.Storage.S3.Endpoint,
		cfg.Storage.S3.AccessKey,
		cfg.Storage.S3.SecretKey,
		cfg.Storage.S3.Bucket,
	)
	if err != nil {
		logger.Fatal("Failed to initialize S3 storage", zap.Error(err))
		return
	}

	speechkitClient := speechkit.NewClient(cfg.STT.Yandex.APIKey, cfg.STT.Yandex.FolderID)

	tb, err := tele.NewBot(tele.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		logger.Fatal("Failed to create Telegram bot", zap.Error(err))
		return
	}

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

	processor := worker.NewProcessor(db, s3Storage, speechkitClient, tb, redisCache)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("Starting to consume messages from queue")
		if err := rabbitMQ.Consume(queue.QueueNameVoiceProcessing, processor.ProcessTask); err != nil {
			logger.Error("Failed to consume messages", zap.Error(err))
			cancel()
		}
	}()

	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	logger.Info("Worker service shutdown complete")
}
