package bot

import (
	"sync"
	"time"
	"voxly/internal/config"
	"voxly/internal/queue"
	"voxly/internal/storage"
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

	// Состояние активности для каждого чата
	activeChatsMu sync.RWMutex
	activeChats   map[int64]bool // chat_id -> is_active
}

func NewBot(cfg *config.Config, db *storage.PostgresStorage, q QueuePublisher) (*Bot, error) {
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
		cfg:         cfg,
		tb:          tb,
		storage:     db,
		q:           q,
		activeChats: make(map[int64]bool),
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

	b.activeChatsMu.Lock()
	b.activeChats[chatID] = true
	b.activeChatsMu.Unlock()

	logger.Info("Bot activated for chat",
		zap.Int64("chat_id", chatID))

	return c.Send("Бот запущен!")
}

// handleStop выключает обработку голосовых сообщений для данного чата
func (b *Bot) handleStop(c tele.Context) error {
	chatID := c.Chat().ID

	b.activeChatsMu.Lock()
	b.activeChats[chatID] = false
	b.activeChatsMu.Unlock()

	logger.Info("Bot deactivated for chat",
		zap.Int64("chat_id", chatID))

	return c.Send("Бот остановлен.\nЧтобы возобновить работу, отправьте /start")
}

// isActive проверяет, активен ли бот для данного чата
func (b *Bot) isActive(chatID int64) bool {
	b.activeChatsMu.RLock()
	defer b.activeChatsMu.RUnlock()

	active, exists := b.activeChats[chatID]
	// По умолчанию бот неактивен, пока не получит /start
	return exists && active
}

func (b *Bot) Start() {
	b.tb.Start()
	logger.Info("Bot started")
}

func (b *Bot) Stop() {
	b.tb.Stop()
	logger.Info("Bot stopped")
}
