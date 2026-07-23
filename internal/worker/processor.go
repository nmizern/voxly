package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"voxly/internal/media"
	"voxly/internal/queue"
	"voxly/internal/storage"
	"voxly/internal/stt"
	"voxly/pkg/cache"
	"voxly/pkg/logger"
	"voxly/pkg/metrics"
	"voxly/pkg/model"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v4"
)

type Processor struct {
	store       storage.Store
	transcriber stt.Transcriber
	bot         *tele.Bot
	cache       cache.Cache
	httpClient  *http.Client
}

func NewProcessor(store storage.Store, transcriber stt.Transcriber, bot *tele.Bot, c cache.Cache) *Processor {
	return &Processor{
		store:       store,
		transcriber: transcriber,
		bot:         bot,
		cache:       c,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *Processor) ProcessTask(taskData []byte) error {
	var vt queue.VoiceTask
	if err := json.Unmarshal(taskData, &vt); err != nil {
		return fmt.Errorf("unmarshal task: %w", err)
	}

	logger.Info("Processing voice task",
		zap.String("task_id", vt.TaskID),
		zap.Int64("chat_id", vt.ChatID))

	kind := vt.Kind
	if kind == "" {
		kind = "voice"
	}
	status := "failed"
	defer func() { metrics.TasksTotal.WithLabelValues(kind, status).Inc() }()

	ctx := context.Background()

	task, err := p.store.GetTaskByID(ctx, vt.TaskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	task.Status = model.TaskStatusInProgress
	task.UpdatedAt = time.Now()
	if err := p.store.UpdateTask(ctx, task); err != nil {
		logger.Error("Failed to mark task in progress", zap.Error(err))
	}

	fileData, err := p.downloadTelegramFile(vt.FileID)
	if err != nil {
		p.handleTaskError(ctx, task, fmt.Sprintf("download file: %v", err))
		return err
	}

	audioData := fileData
	filename := "voice" + extForMIME(vt.MimeType)
	mime := vt.MimeType

	if vt.Kind == "video_note" {
		extracted, err := media.ExtractAudio(ctx, fileData)
		if err != nil {
			p.handleTaskError(ctx, task, fmt.Sprintf("extract audio: %v", err))
			return err
		}
		audioData = extracted
		filename = "video.ogg"
		mime = "audio/ogg"
	}

	start := time.Now()
	result, err := p.transcriber.Transcribe(ctx, stt.Audio{
		Data:     bytes.NewReader(audioData),
		Filename: filename,
		MIME:     mime,
		Duration: vt.Duration,
	})
	if err != nil {
		p.handleTaskError(ctx, task, fmt.Sprintf("transcribe: %v", err))
		return err
	}
	metrics.TranscriptionSeconds.WithLabelValues(p.transcriber.Name()).Observe(time.Since(start).Seconds())

	text := strings.TrimSpace(result.Text)
	if text == "" {
		p.handleTaskError(ctx, task, "no text recognized")
		return fmt.Errorf("no text recognized")
	}

	logger.Info("Recognition completed",
		zap.String("task_id", task.ID),
		zap.Int("text_length", len(text)))

	transcript := &model.Transcript{
		ID:          uuid.NewString(),
		TaskID:      task.ID,
		Text:        text,
		RawResponse: result.Raw,
		CreatedAt:   time.Now(),
	}
	if err := p.store.CreateTranscript(ctx, transcript); err != nil {
		logger.Error("Failed to save transcript", zap.Error(err))
	}

	if err := p.cache.SetWithTTL(ctx, cache.TranscriptCacheKey(task.ID), transcript, 7*24*time.Hour); err != nil {
		logger.Error("Failed to cache transcript", zap.Error(err))
	}

	task.SetCompleted()
	if err := p.store.UpdateTask(ctx, task); err != nil {
		logger.Error("Failed to mark task done", zap.Error(err))
	}

	if err := p.sendResultToUser(vt.ChatID, vt.TelegramMessageID, text); err != nil {
		logger.Error("Failed to send result to user", zap.Error(err))
	}

	status = "done"
	logger.Info("Task completed", zap.String("task_id", task.ID))
	return nil
}

func (p *Processor) downloadTelegramFile(fileID string) ([]byte, error) {
	file, err := p.bot.FileByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("get file info: %w", err)
	}

	fileURL := p.bot.URL + "/file/bot" + p.bot.Token + "/" + file.FilePath

	resp, err := p.httpClient.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download file: status=%d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (p *Processor) sendResultToUser(chatID, replyToMessageID int64, text string) error {
	chat := &tele.Chat{ID: chatID}
	_, err := p.bot.Send(chat, text, &tele.SendOptions{
		ReplyTo: &tele.Message{ID: int(replyToMessageID)},
	})
	return err
}

func (p *Processor) handleTaskError(ctx context.Context, task *model.Task, errorMsg string) {
	logger.Error("Task processing error",
		zap.String("task_id", task.ID),
		zap.String("error", errorMsg))

	task.SetError(errorMsg)
	task.IncrementAttempts()

	if err := p.store.UpdateTask(ctx, task); err != nil {
		logger.Error("Failed to update task error", zap.Error(err))
	}

	if task.Attempts >= 3 {
		chat := &tele.Chat{ID: task.ChatID}
		p.bot.Send(chat, "Не удалось распознать сообщение после нескольких попыток.", &tele.SendOptions{
			ReplyTo: &tele.Message{ID: int(task.TelegramMessageID)},
		})
	}
}

func extForMIME(mime string) string {
	switch mime {
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	default:
		return ".ogg"
	}
}
