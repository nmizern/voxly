package app

import (
	"context"
	"fmt"
	"sync"
	"time"
	"voxly/internal/bot"
	"voxly/internal/config"
	"voxly/internal/queue"
	"voxly/internal/storage"
	"voxly/internal/stt"
	"voxly/internal/worker"
	"voxly/pkg/cache"
	"voxly/pkg/logger"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v4"
)

type App struct {
	cfg       *config.Config
	bot       *bot.Bot
	processor *worker.Processor
	queue     queue.Queue
	store     storage.Store
	cache     cache.Cache
}

func New(cfg *config.Config) (*App, error) {
	c, err := buildCache(cfg)
	if err != nil {
		return nil, err
	}
	store, err := buildStore(cfg)
	if err != nil {
		return nil, err
	}
	q, err := buildQueue(cfg)
	if err != nil {
		return nil, err
	}
	transcriber, err := buildTranscriber(cfg)
	if err != nil {
		return nil, err
	}

	tb, err := tele.NewBot(tele.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &App{
		cfg:       cfg,
		bot:       bot.NewBot(cfg, tb, store, q, c),
		processor: worker.NewProcessor(store, transcriber, tb, c),
		queue:     q,
		store:     store,
		cache:     c,
	}, nil
}

// Run starts the bot and/or worker according to the configured role and blocks
// until ctx is cancelled. Lite mode always runs both in one process.
func (a *App) Run(ctx context.Context) error {
	role := a.cfg.Role
	if a.cfg.Mode == config.ModeLite {
		role = "all"
	}

	var wg sync.WaitGroup

	if role == "all" || role == "bot" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("Starting Telegram bot")
			a.bot.Start()
		}()
	}

	if role == "all" || role == "worker" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("Starting worker", zap.Int("concurrency", a.cfg.Worker.Concurrency))
			if err := a.queue.Consume(queue.QueueNameVoiceProcessing, a.cfg.Worker.Concurrency, a.processor.ProcessTask); err != nil {
				logger.Error("Worker stopped", zap.Error(err))
			}
		}()
	}

	<-ctx.Done()
	logger.Info("Shutting down")

	if role == "all" || role == "bot" {
		a.bot.Stop()
	}
	_ = a.queue.Close()
	wg.Wait()
	return nil
}

func (a *App) Close() {
	_ = a.queue.Close()
	_ = a.cache.Close()
	_ = a.store.Close()
}

func buildCache(cfg *config.Config) (cache.Cache, error) {
	switch cfg.Cache.Driver {
	case "redis":
		return cache.NewRedisCache(cfg.Cache.Redis.Addr, cfg.Cache.Redis.Password, cfg.Cache.Redis.DB, 24*time.Hour)
	case "memory":
		return cache.NewMemoryCache(24 * time.Hour), nil
	default:
		return nil, fmt.Errorf("unknown cache driver %q", cfg.Cache.Driver)
	}
}

func buildStore(cfg *config.Config) (storage.Store, error) {
	switch cfg.Database.Driver {
	case "postgres":
		return storage.NewPostgresStorage(cfg.Database.DSN)
	case "memory":
		return storage.NewMemoryStore(), nil
	default:
		return nil, fmt.Errorf("unknown database driver %q", cfg.Database.Driver)
	}
}

func buildQueue(cfg *config.Config) (queue.Queue, error) {
	switch cfg.Queue.Driver {
	case "rabbitmq":
		return queue.NewRabbitMQ(cfg.Queue.RabbitMQ.URL)
	case "memory":
		return queue.NewMemoryQueue(0), nil
	default:
		return nil, fmt.Errorf("unknown queue driver %q", cfg.Queue.Driver)
	}
}

func buildTranscriber(cfg *config.Config) (stt.Transcriber, error) {
	var store stt.ObjectStore
	if cfg.STT.Provider == config.ProviderYandex {
		s3, err := storage.NewS3Storage(cfg.Storage.S3.Endpoint, cfg.Storage.S3.AccessKey, cfg.Storage.S3.SecretKey, cfg.Storage.S3.Bucket)
		if err != nil {
			return nil, err
		}
		store = s3
	}
	return stt.New(cfg, store)
}
