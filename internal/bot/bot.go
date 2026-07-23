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

type Bot struct {
	cfg   *config.Config
	tb    *tele.Bot
	q     queue.Publisher
	store storage.Store
	cache cache.Cache
}

func NewBot(cfg *config.Config, tb *tele.Bot, store storage.Store, q queue.Publisher, c cache.Cache) *Bot {
	b := &Bot{
		cfg:   cfg,
		tb:    tb,
		q:     q,
		store: store,
		cache: c,
	}
	b.registerHandlers()
	return b
}

func (b *Bot) registerHandlers() {
	b.tb.Handle("/start", b.handleStart)
	b.tb.Handle("/stop", b.handleStop)
	b.tb.Handle(tele.OnVoice, b.handleVoice)
	b.tb.Handle(tele.OnVideoNote, b.handleVideoNote)
}

// handleStart включает обработку голосовых сообщений для данного чата
func (b *Bot) handleStart(c tele.Context) error {
	if !b.allowed(c) {
		return nil
	}
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
	if !b.allowed(c) {
		return nil
	}
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

// allowed reports whether the sender may use the bot. In "allowlist" mode only
// admins and listed users/chats pass.
func (b *Bot) allowed(c tele.Context) bool {
	if b.cfg.Access.Mode != "allowlist" {
		return true
	}

	var userID int64
	if u := c.Sender(); u != nil {
		userID = u.ID
	}
	if b.cfg.IsAdmin(userID) {
		return true
	}
	for _, id := range b.cfg.Access.AllowedUsers {
		if id == userID {
			return true
		}
	}
	for _, id := range b.cfg.Access.AllowedChats {
		if id == c.Chat().ID {
			return true
		}
	}
	return false
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
