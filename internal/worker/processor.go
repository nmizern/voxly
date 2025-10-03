package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"voxly/internal/queue"
	"voxly/internal/speechkit"
	"voxly/internal/storage"
	"voxly/pkg/logger"
	"voxly/pkg/model"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v4"
)

type Processor struct {
	db         *storage.PostgresStorage
	s3         *storage.S3Storage
	speechkit  *speechkit.Client
	bot        *tele.Bot
	httpClient *http.Client
}

// NewProcessor creates a new worker processor
func NewProcessor(
	db *storage.PostgresStorage,
	s3 *storage.S3Storage,
	speechkitClient *speechkit.Client,
	bot *tele.Bot,
) *Processor {
	return &Processor{
		db:        db,
		s3:        s3,
		speechkit: speechkitClient,
		bot:       bot,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ProcessTask processes a voice message task
func (p *Processor) ProcessTask(taskData []byte) error {
	var voiceTask queue.VoiceTask
	if err := json.Unmarshal(taskData, &voiceTask); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	logger.Info("Processing voice task",
		zap.String("task_id", voiceTask.TaskID),
		zap.Int64("chat_id", voiceTask.ChatID))

	ctx := context.Background()

	// Get task from database
	task, err := p.db.GetTaskByID(ctx, voiceTask.TaskID)
	if err != nil {
		return fmt.Errorf("failed to get task from db: %w", err)
	}

	// Update task status to in_progress
	task.SetInProgress("")
	if err := p.db.UpdateTask(ctx, task); err != nil {
		logger.Error("Failed to update task status", zap.Error(err))
	}

	// Download file from Telegram
	fileData, err := p.downloadTelegramFile(voiceTask.FileID)
	if err != nil {
		p.handleTaskError(ctx, task, fmt.Sprintf("Failed to download file: %v", err))
		return err
	}

	logger.Info("File downloaded from Telegram",
		zap.String("task_id", task.ID),
		zap.Int("size", len(fileData)))

	// Upload to S3
	s3Key := p.s3.GenerateKey(task.ID, ".ogg")
	s3URL, err := p.s3.UploadFile(ctx, s3Key, bytes.NewReader(fileData), "audio/ogg")
	if err != nil {
		p.handleTaskError(ctx, task, fmt.Sprintf("Failed to upload to S3: %v", err))
		return err
	}

	logger.Info("File uploaded to S3",
		zap.String("task_id", task.ID),
		zap.String("s3_url", s3URL))

	// Start speech recognition
	operationID, err := p.speechkit.StartRecognition(s3URL)
	if err != nil {
		p.handleTaskError(ctx, task, fmt.Sprintf("Failed to start recognition: %v", err))
		return err
	}

	task.OperationID = &operationID
	if err := p.db.UpdateTask(ctx, task); err != nil {
		logger.Error("Failed to update operation_id", zap.Error(err))
	}

	logger.Info("Recognition started",
		zap.String("task_id", task.ID),
		zap.String("operation_id", operationID))

	// Wait for recognition result
	result, err := p.speechkit.WaitForResult(operationID)
	if err != nil {
		p.handleTaskError(ctx, task, fmt.Sprintf("Recognition failed: %v", err))
		return err
	}

	// Extract text
	recognizedText := result.GetFullText()
	if recognizedText == "" {
		p.handleTaskError(ctx, task, "No text recognized")
		return fmt.Errorf("no text recognized")
	}

	logger.Info("Recognition completed",
		zap.String("task_id", task.ID),
		zap.Int("text_length", len(recognizedText)))

	// Save transcript to database
	rawResponse, _ := json.Marshal(result)
	transcript := &model.Transcript{
		ID:          uuid.New().String(),
		TaskID:      task.ID,
		Text:        recognizedText,
		RawResponse: rawResponse,
		CreatedAt:   time.Now(),
	}

	if err := p.db.CreateTranscript(ctx, transcript); err != nil {
		logger.Error("Failed to save transcript", zap.Error(err))
	}

	// Update task status to done
	task.SetCompleted()
	if err := p.db.UpdateTask(ctx, task); err != nil {
		logger.Error("Failed to update task status to done", zap.Error(err))
	}

	// Send result back to user
	if err := p.sendResultToUser(voiceTask.ChatID, voiceTask.TelegramMessageID, recognizedText); err != nil {
		logger.Error("Failed to send result to user", zap.Error(err))
		// Don't return error - task is completed anyway
	}

	logger.Info("Task completed successfully",
		zap.String("task_id", task.ID))

	return nil
}

// downloadTelegramFile downloads file from Telegram
func (p *Processor) downloadTelegramFile(fileID string) ([]byte, error) {
	file, err := p.bot.FileByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileURL := p.bot.URL + "/file/bot" + p.bot.Token + "/" + file.FilePath

	resp, err := p.httpClient.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file: status=%d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	return data, nil
}

// sendResultToUser sends recognition result back to user
func (p *Processor) sendResultToUser(chatID, replyToMessageID int64, text string) error {
	chat := &tele.Chat{ID: chatID}
	

	_, err := p.bot.Send(chat, text, &tele.SendOptions{
		ReplyTo: &tele.Message{ID: int(replyToMessageID)},
	})

	return err
}

// handleTaskError handles task error
func (p *Processor) handleTaskError(ctx context.Context, task *model.Task, errorMsg string) {
	logger.Error("Task processing error",
		zap.String("task_id", task.ID),
		zap.String("error", errorMsg))

	task.SetError(errorMsg)
	task.IncrementAttempts()

	if err := p.db.UpdateTask(ctx, task); err != nil {
		logger.Error("Failed to update task error", zap.Error(err))
	}

	// Optionally notify user about error
	if task.Attempts >= 3 {
		chat := &tele.Chat{ID: task.ChatID}
		message := "Не удалось распознать голосовое сообщение после нескольких попыток."
		p.bot.Send(chat, message, &tele.SendOptions{
			ReplyTo: &tele.Message{ID: int(task.TelegramMessageID)},
		})
	}
}
