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

func mediaFromMessage(msg *tele.Message) (mediaTask, bool) {
	if msg == nil {
		return mediaTask{}, false
	}
	if msg.Voice != nil {
		return mediaTask{
			fileID:   msg.Voice.FileID,
			kind:     "voice",
			duration: msg.Voice.Duration,
			fileSize: int64(msg.Voice.FileSize),
			mime:     msg.Voice.MIME,
		}, true
	}
	if msg.VideoNote != nil {
		return mediaTask{
			fileID:   msg.VideoNote.FileID,
			kind:     "video_note",
			duration: msg.VideoNote.Duration,
			fileSize: int64(msg.VideoNote.FileSize),
			mime:     "video/mp4",
		}, true
	}
	return mediaTask{}, false
}

func (b *Bot) handleMedia(c tele.Context) error {
	if !b.autoEnabled() {
		return nil
	}
	m, ok := mediaFromMessage(c.Message())
	if !ok {
		return nil
	}
	return b.enqueue(c, c.Message(), m, false)
}

func (b *Bot) handleTranscribe(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.ReplyTo == nil {
		return c.Reply("Ответьте этой командой на голосовое или видеосообщение")
	}
	m, ok := mediaFromMessage(msg.ReplyTo)
	if !ok {
		return c.Reply("Это не голосовое и не видеосообщение")
	}
	return b.enqueue(c, msg.ReplyTo, m, true)
}

func (b *Bot) enqueue(c tele.Context, src *tele.Message, m mediaTask, explicit bool) error {
	chat := c.Chat()
	if chat == nil || src == nil {
		return nil
	}

	if !b.allowed(c) {
		logger.Info("Access denied", zap.Int64("chat_id", chat.ID))
		if explicit || chat.Type == tele.ChatPrivate {
			return c.Reply("У вас нет доступа к этому боту.")
		}
		return nil
	}

	// Auto in groups only after /start. /transcribe is always on.
	if !explicit && chat.Type != tele.ChatPrivate && !b.isActive(chat.ID) {
		logger.Info("Ignoring message from inactive chat",
			zap.Int64("chat_id", chat.ID),
			zap.Int("message_id", src.ID))
		return nil
	}

	if !b.withinQuota(c) {
		if explicit || chat.Type == tele.ChatPrivate {
			return c.Reply("Дневной лимит запросов исчерпан, попробуйте завтра.")
		}
		return nil
	}

	if err := c.Reply("Обработка..."); err != nil {
		logger.Error("Failed to send processing message", zap.Error(err))
	}

	task := model.Task{
		ID:                uuid.New().String(),
		TelegramMessageID: int64(src.ID),
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
