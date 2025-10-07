package bot

import (
	"context"
	"time"
	"voxly/internal/config"
	"voxly/internal/queue"
	"voxly/internal/storage"
	"voxly/pkg/cache"
	"voxly/pkg/logger"

	tele "gopkg.in/telebot.v4"

	"go.uber.org/zap"
)

type QueuePublisher interface {
	Publish(queueName string, body []byte) error
	PublishTask(task *queue.VoiceTask) error
}

type Bot struct {
	cfg     *config.Config
	tb      *tele.Bot
	q       QueuePublisher
	storage *storage.PostgresStorage
	cache   cache.Cache
}

func NewBot(cfg *config.Config, db *storage.PostgresStorage, q QueuePublisher, redisCache cache.Cache) (*Bot, error) {
	logger.Info("Starting bot initialization")

	pref := tele.Settings{
		Token: cfg.Telegram.Token,
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	}

	if pref.Token == "" {
		logger.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
		return nil, nil
	}

	tb, err := tele.NewBot(pref)
	if err != nil {
		logger.Fatal("Failed to create bot", zap.Error(err))
		return nil, err
	}

	logger.Info("Bot created successfully")

	bot := &Bot{
		cfg:     cfg,
		tb:      tb,
		storage: db,
		q:       q,
		cache:   redisCache,
	}

	bot.registerHandlers()
	return bot, nil
}

func (b *Bot) registerHandlers() {
	b.tb.Handle("/start", b.handleStart)
	b.tb.Handle("/stop", b.handleStop)
	b.tb.Handle(tele.OnVoice, b.handleVoice)
}

// handleStart включает обработку голосовых сообщений для данного чата
func (b *Bot) handleStart(c tele.Context) error {
	chatID := c.Chat().ID
	ctx := context.Background()

	// Сохраняем в Redis с TTL 30 дней
	key := cache.ChatActiveCacheKey(chatID)
	if err := b.cache.SetWithTTL(ctx, key, "true", 30*24*time.Hour); err != nil {
		logger.Error("Failed to save chat active state to cache", zap.Error(err))
	}

	logger.Info("Bot activated for chat",
		zap.Int64("chat_id", chatID))

	return c.Send("Бот запущен!")
}

// handleStop выключает обработку голосовых сообщений для данного чата
func (b *Bot) handleStop(c tele.Context) error {
	chatID := c.Chat().ID
	ctx := context.Background()

	// Удаляем из Redis
	key := cache.ChatActiveCacheKey(chatID)
	if err := b.cache.Delete(ctx, key); err != nil {
		logger.Error("Failed to delete chat active state from cache", zap.Error(err))
	}

	logger.Info("Bot deactivated for chat",
		zap.Int64("chat_id", chatID))

	return c.Send("Бот остановлен.\nЧтобы возобновить работу, отправьте /start")
}

// isActive проверяет, активен ли бот для данного чата
func (b *Bot) isActive(chatID int64) bool {
	ctx := context.Background()
	key := cache.ChatActiveCacheKey(chatID)

	var value string
	err := b.cache.Get(ctx, key, &value)
	if err != nil {
		// Ключ не найден или ошибка - бот неактивен
		return false
	}

	// Проверяем значение
	return value == "true"
}

func (b *Bot) Start() {
	b.tb.Start()
	logger.Info("Bot started")
}

func (b *Bot) Stop() {
	b.tb.Stop()
	logger.Info("Bot stopped")
}
