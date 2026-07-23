package bot

import (
	"context"
	"fmt"
	"time"
	"voxly/internal/queue"
	"voxly/pkg/logger"
	"voxly/pkg/model"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v4"
)

type mediaTask struct {
	fileID   string
	kind     string
	duration int
	fileSize int64
	mime     string
}

// withinQuota enforces a per user daily limit. Admins and a zero limit are
// always allowed; on a cache error it fails open.
func (b *Bot) withinQuota(c tele.Context) bool {
	limit := b.cfg.Access.UserDailyLimit
	if limit <= 0 {
		return true
	}
	u := c.Sender()
	if u == nil || b.cfg.IsAdmin(u.ID) {
		return true
	}

	key := fmt.Sprintf("quota:%d:%s", u.ID, time.Now().UTC().Format("2006-01-02"))
	n, err := b.cache.Increment(context.Background(), key, 24*time.Hour)
	if err != nil {
		logger.Error("Quota check failed", zap.Error(err))
		return true
	}
	return n <= int64(limit)
}

func (b *Bot) handleVoice(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Voice == nil {
		return c.Reply("Ошибка: голосовое сообщение не найдено")
	}
	return b.enqueue(c, mediaTask{
		fileID:   msg.Voice.FileID,
		kind:     "voice",
		duration: msg.Voice.Duration,
		fileSize: int64(msg.Voice.FileSize),
		mime:     msg.Voice.MIME,
	})
}

func (b *Bot) handleVideoNote(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.VideoNote == nil {
		return c.Reply("Ошибка: видеосообщение не найдено")
	}
	return b.enqueue(c, mediaTask{
		fileID:   msg.VideoNote.FileID,
		kind:     "video_note",
		duration: msg.VideoNote.Duration,
		fileSize: int64(msg.VideoNote.FileSize),
		mime:     "video/mp4",
	})
}

func (b *Bot) enqueue(c tele.Context, m mediaTask) error {
	msg := c.Message()
	chat := msg.Chat

	if !b.allowed(c) {
		logger.Info("Access denied", zap.Int64("chat_id", chat.ID))
		if chat.Type == tele.ChatPrivate {
			return c.Reply("У вас нет доступа к этому боту.")
		}
		return nil
	}

	// Private chats are always on, groups require /start.
	if chat.Type != tele.ChatPrivate && !b.isActive(chat.ID) {
		logger.Info("Ignoring message from inactive chat",
			zap.Int64("chat_id", chat.ID),
			zap.Int("message_id", msg.ID))
		return nil
	}

	if !b.withinQuota(c) {
		if chat.Type == tele.ChatPrivate {
			return c.Reply("Дневной лимит запросов исчерпан, попробуйте завтра.")
		}
		return nil
	}

	if err := c.Reply("Обработка..."); err != nil {
		logger.Error("Failed to send processing message", zap.Error(err))
	}

	task := model.Task{
		ID:                uuid.New().String(),
		TelegramMessageID: int64(msg.ID),
		ChatID:            chat.ID,
		FileID:            m.fileID,
		Status:            model.TaskStatusQueued,
		Meta: model.JSONB{
			"kind":      m.kind,
			"duration":  m.duration,
			"file_size": m.fileSize,
			"mime_type": m.mime,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	if err := b.store.CreateTask(ctx, &task); err != nil {
		logger.Error("Failed to create task", zap.Error(err), zap.String("task_id", task.ID))
		return c.Reply("Ошибка при сохранении задачи")
	}

	logger.Info("Task created",
		zap.String("task_id", task.ID),
		zap.String("kind", m.kind),
		zap.Int64("chat_id", task.ChatID))

	if err := b.q.PublishTask(&queue.VoiceTask{
		TaskID:            task.ID,
		ChatID:            task.ChatID,
		TelegramMessageID: task.TelegramMessageID,
		FileID:            task.FileID,
		Kind:              m.kind,
		Duration:          m.duration,
		FileSize:          m.fileSize,
		MimeType:          m.mime,
		CreatedAt:         task.CreatedAt,
	}); err != nil {
		logger.Error("Failed to publish task", zap.Error(err), zap.String("task_id", task.ID))
		return c.Reply("Ошибка при отправке задачи в очередь")
	}

	return nil
}
