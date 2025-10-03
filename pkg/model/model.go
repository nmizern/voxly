package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusFailed     TaskStatus = "failed"
)

// JSONB represents a JSONB field for PostgreSQL
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Task represents a voice message processing task
type Task struct {
	ID                string     `json:"id" db:"id"`
	TelegramMessageID int64      `json:"telegram_message_id" db:"telegram_message_id"`
	ChatID            int64      `json:"chat_id" db:"chat_id"`
	FileID            string     `json:"file_id" db:"file_id"`
	Status            TaskStatus `json:"status" db:"status"`
	OperationID       *string    `json:"operation_id,omitempty" db:"operation_id"`
	Attempts          int        `json:"attempts" db:"attempts"`
	ErrorText         *string    `json:"error_text,omitempty" db:"error_text"`
	Meta              JSONB      `json:"meta" db:"meta"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// Transcript represents a transcribed text result
type Transcript struct {
	ID          string          `json:"id" db:"id"`
	TaskID      string          `json:"task_id" db:"task_id"`
	Text        string          `json:"text" db:"text"`
	RawResponse json.RawMessage `json:"raw_response,omitempty" db:"raw_response"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

// IsCompleted returns true if the task is in a final state
func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusDone || t.Status == TaskStatusFailed
}

// CanRetry returns true if the task can be retried
func (t *Task) CanRetry() bool {
	return t.Status == TaskStatusFailed && t.Attempts < 3 // максимум 3 попытки
}

// IncrementAttempts increases the attempt counter
func (t *Task) IncrementAttempts() {
	t.Attempts++
}

// SetError sets the task status to failed with error message
func (t *Task) SetError(errorText string) {
	t.Status = TaskStatusFailed
	t.ErrorText = &errorText
	t.UpdatedAt = time.Now()
}

// SetCompleted sets the task status to done
func (t *Task) SetCompleted() {
	t.Status = TaskStatusDone
	t.UpdatedAt = time.Now()
}

// SetInProgress sets the task status to in progress with operation ID
func (t *Task) SetInProgress(operationID string) {
	t.Status = TaskStatusInProgress
	t.OperationID = &operationID
	t.UpdatedAt = time.Now()
}
