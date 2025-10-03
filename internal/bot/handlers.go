package bot

import (
	"context"
	"time"
	"voxly/internal/queue"
	"voxly/pkg/logger"
	"voxly/pkg/model"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v4"
)

func (b *Bot) handleVoice(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Voice == nil {
		return c.Reply("Ошибка: голосовое сообщение не найдено")
	}

	// Check if bot is active for this chat
	if !b.isActive(msg.Chat.ID) {
		logger.Info("Ignoring voice message from inactive chat",
			zap.Int64("chat_id", msg.Chat.ID),
			zap.Int("message_id", msg.ID))

		return nil
	}

	if err := c.Reply("Обработка..."); err != nil {
		logger.Error("Failed to send processing message", zap.Error(err))
	}

	// Creating task
	task := model.Task{
		ID:                uuid.New().String(),
		TelegramMessageID: int64(msg.ID),
		ChatID:            msg.Chat.ID,
		FileID:            msg.Voice.FileID,
		Status:            model.TaskStatusQueued,
		OperationID:       nil,
		Attempts:          0,
		ErrorText:         nil,
		Meta: model.JSONB{
			"voice_duration": msg.Voice.Duration,
			"file_size":      msg.Voice.FileSize,
			"mime_type":      msg.Voice.MIME,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Saving task to database
	ctx := context.Background()
	if err := b.storage.CreateTask(ctx, &task); err != nil {
		logger.Error("Failed to create task in database",
			zap.Error(err),
			zap.String("task_id", task.ID))
		return c.Reply("Ошибка при сохранении задачи")
	}

	logger.Info("Task created in database",
		zap.String("task_id", task.ID),
		zap.Int64("telegram_message_id", task.TelegramMessageID),
		zap.Int64("chat_id", task.ChatID))

	// Sending task to RabbitMQ
	if b.q != nil {
		voiceTask := &queue.VoiceTask{
			TaskID:            task.ID,
			ChatID:            task.ChatID,
			TelegramMessageID: task.TelegramMessageID,
			FileID:            task.FileID,
			Duration:          msg.Voice.Duration,
			FileSize:          int64(msg.Voice.FileSize),
			MimeType:          msg.Voice.MIME,
			CreatedAt:         task.CreatedAt,
		}

		if err := b.q.PublishTask(voiceTask); err != nil {
			logger.Error("Failed to publish task to queue",
				zap.Error(err),
				zap.String("task_id", task.ID))
			return c.Reply("Ошибка при отправке задачи в очередь")
		}

		logger.Info("Task published to queue",
			zap.String("task_id", task.ID))
	}

	return nil
}
