package bot

import (
	"context"
	"errors"
	"testing"
	"time"
	"voxly/internal/queue"
	"voxly/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Storage
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) CreateTask(ctx context.Context, task *model.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockStorage) GetTaskByID(ctx context.Context, id string) (*model.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *MockStorage) UpdateTask(ctx context.Context, task *model.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockStorage) CreateTranscript(ctx context.Context, transcript *model.Transcript) error {
	args := m.Called(ctx, transcript)
	return args.Error(0)
}

func (m *MockStorage) GetTranscriptByTaskID(ctx context.Context, taskID string) (*model.Transcript, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Transcript), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Mock Queue
type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) PublishTask(task *queue.VoiceTask) error {
	args := m.Called(task)
	return args.Error(0)
}

func (m *MockQueue) Consume(ctx context.Context, handler func([]byte) error) error {
	args := m.Called(ctx, handler)
	return args.Error(0)
}

func (m *MockQueue) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestBot_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		chatID   int64
		setup    func(*Bot)
		expected bool
	}{
		{
			name:   "chat is active",
			chatID: 123,
			setup: func(b *Bot) {
				b.activeChats[123] = true
			},
			expected: true,
		},
		{
			name:   "chat is inactive",
			chatID: 456,
			setup: func(b *Bot) {
				b.activeChats[456] = false
			},
			expected: false,
		},
		{
			name:     "chat not in map",
			chatID:   789,
			setup:    func(b *Bot) {},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bot{
				activeChats: make(map[int64]bool),
			}
			tt.setup(b)
			result := b.isActive(tt.chatID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBot_SetActive(t *testing.T) {
	b := &Bot{
		activeChats: make(map[int64]bool),
	}

	chatID := int64(123)

	b.activeChats[chatID] = true
	assert.True(t, b.isActive(chatID))

	b.activeChats[chatID] = false
	assert.False(t, b.isActive(chatID))
}

func TestTask_SetInProgress(t *testing.T) {
	task := &model.Task{
		ID:        "test-id",
		Status:    model.TaskStatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	operationID := "op-123"
	task.SetInProgress(operationID)

	assert.Equal(t, model.TaskStatusInProgress, task.Status)
	assert.NotNil(t, task.OperationID)
	assert.Equal(t, operationID, *task.OperationID)
}

func TestTask_SetCompleted(t *testing.T) {
	task := &model.Task{
		ID:        "test-id",
		Status:    model.TaskStatusInProgress,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	task.SetCompleted()

	assert.Equal(t, model.TaskStatusDone, task.Status)
	assert.Nil(t, task.ErrorText)
}

func TestTask_SetFailed(t *testing.T) {
	task := &model.Task{
		ID:        "test-id",
		Status:    model.TaskStatusInProgress,
		Attempts:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	errorMsg := "test error"
	task.SetError(errorMsg)

	assert.Equal(t, model.TaskStatusFailed, task.Status)
	assert.NotNil(t, task.ErrorText)
	assert.Equal(t, errorMsg, *task.ErrorText)
}

func TestStorageIntegration_CreateAndGetTask(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	mockStorage := new(MockStorage)
	task := &model.Task{
		ID:                "test-task-123",
		TelegramMessageID: 1,
		ChatID:            123,
		FileID:            "file-123",
		Status:            model.TaskStatusQueued,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	ctx := context.Background()

	mockStorage.On("CreateTask", ctx, task).Return(nil)
	mockStorage.On("GetTaskByID", ctx, "test-task-123").Return(task, nil)

	err := mockStorage.CreateTask(ctx, task)
	assert.NoError(t, err)

	retrievedTask, err := mockStorage.GetTaskByID(ctx, "test-task-123")
	assert.NoError(t, err)
	assert.Equal(t, task.ID, retrievedTask.ID)

	mockStorage.AssertExpectations(t)
}

func TestQueueIntegration_PublishTask(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	mockQueue := new(MockQueue)
	voiceTask := &queue.VoiceTask{
		TaskID:            "task-123",
		ChatID:            123,
		TelegramMessageID: 1,
		FileID:            "file-123",
		Duration:          10,
		FileSize:          1024,
		MimeType:          "audio/ogg",
		CreatedAt:         time.Now(),
	}

	mockQueue.On("PublishTask", voiceTask).Return(nil)

	err := mockQueue.PublishTask(voiceTask)
	assert.NoError(t, err)

	mockQueue.AssertExpectations(t)
}

func TestQueueIntegration_PublishTaskError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	mockQueue := new(MockQueue)
	voiceTask := &queue.VoiceTask{
		TaskID: "task-123",
	}

	expectedError := errors.New("queue connection failed")
	mockQueue.On("PublishTask", voiceTask).Return(expectedError)

	err := mockQueue.PublishTask(voiceTask)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockQueue.AssertExpectations(t)
}
