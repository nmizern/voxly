package queue

import "time"

// VoiceTask represents a voice message processing task
type VoiceTask struct {
	TaskID            string    `json:"task_id"`
	ChatID            int64     `json:"chat_id"`
	TelegramMessageID int64     `json:"telegram_message_id"`
	FileID            string    `json:"file_id"`
	Duration          int       `json:"duration"`
	FileSize          int64     `json:"file_size"`
	MimeType          string    `json:"mime_type"`
	CreatedAt         time.Time `json:"created_at"`
}

// TranscriptionResult represents the result of speech recognition
type TranscriptionResult struct {
	TaskID       string `json:"task_id"`
	Text         string `json:"text"`
	RawResponse  []byte `json:"raw_response,omitempty"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}
