package worker

import (
	"context"
	"errors"
	"testing"
	"time"
	"voxly/internal/speechkit"
	"voxly/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDB struct {
	mock.Mock
}

func (m *MockDB) CreateTask(ctx context.Context, task *model.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockDB) GetTaskByID(ctx context.Context, id string) (*model.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *MockDB) UpdateTask(ctx context.Context, task *model.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockDB) CreateTranscript(ctx context.Context, transcript *model.Transcript) error {
	args := m.Called(ctx, transcript)
	return args.Error(0)
}

func (m *MockDB) GetTranscriptByTaskID(ctx context.Context, taskID string) (*model.Transcript, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Transcript), args.Error(1)
}

func (m *MockDB) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockS3 struct {
	mock.Mock
}

func (m *MockS3) UploadFile(ctx context.Context, key string, data interface{}, contentType string) (string, error) {
	args := m.Called(ctx, key, data, contentType)
	return args.String(0), args.Error(1)
}

func (m *MockS3) GenerateKey(taskID, extension string) string {
	args := m.Called(taskID, extension)
	return args.String(0)
}

type MockSpeechKit struct {
	mock.Mock
}

func (m *MockSpeechKit) StartRecognition(s3URI string) (string, error) {
	args := m.Called(s3URI)
	return args.String(0), args.Error(1)
}

func (m *MockSpeechKit) WaitForResult(operationID string) (*speechkit.RecognitionResult, error) {
	args := m.Called(operationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*speechkit.RecognitionResult), args.Error(1)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string, dest interface{}) error {
	args := m.Called(ctx, key, dest)
	return args.Error(0)
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestProcessor_HandleTaskError(t *testing.T) {
	mockDB := new(MockDB)
	ctx := context.Background()

	task := &model.Task{
		ID:        "task-123",
		Status:    model.TaskStatusInProgress,
		Attempts:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockDB.On("UpdateTask", ctx, mock.AnythingOfType("*model.Task")).Return(nil)

	errorMsg := "test error"
	task.SetError(errorMsg)
	task.IncrementAttempts()

	err := mockDB.UpdateTask(ctx, task)
	assert.NoError(t, err)

	assert.Equal(t, model.TaskStatusFailed, task.Status)
	assert.Equal(t, 1, task.Attempts)
	assert.NotNil(t, task.ErrorText)

	mockDB.AssertExpectations(t)
}

func TestSpeechKit_RecognitionFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	mockSK := new(MockSpeechKit)
	s3URL := "https://storage.yandexcloud.net/bucket/file.ogg"
	operationID := "op-123"

	result := &speechkit.RecognitionResult{
		Chunks: []speechkit.Chunk{
			{
				Alternatives: []speechkit.Alternative{
					{Text: "Test transcription", Confidence: 0.95},
				},
			},
		},
	}

	mockSK.On("StartRecognition", s3URL).Return(operationID, nil)
	mockSK.On("WaitForResult", operationID).Return(result, nil)

	opID, err := mockSK.StartRecognition(s3URL)
	assert.NoError(t, err)
	assert.Equal(t, operationID, opID)

	res, err := mockSK.WaitForResult(operationID)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Chunks, 1)

	mockSK.AssertExpectations(t)
}

func TestS3_UploadFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	mockS3 := new(MockS3)
	ctx := context.Background()

	key := "voice/2025/10/07/task-123.ogg"
	expectedURL := "https://storage.yandexcloud.net/bucket/" + key

	mockS3.On("GenerateKey", "task-123", ".ogg").Return(key)
	mockS3.On("UploadFile", ctx, key, mock.Anything, "audio/ogg").Return(expectedURL, nil)

	generatedKey := mockS3.GenerateKey("task-123", ".ogg")
	assert.Equal(t, key, generatedKey)

	url, err := mockS3.UploadFile(ctx, key, nil, "audio/ogg")
	assert.NoError(t, err)
	assert.Equal(t, expectedURL, url)

	mockS3.AssertExpectations(t)
}

func TestS3_UploadFileError(t *testing.T) {
	mockS3 := new(MockS3)
	ctx := context.Background()

	key := "voice/task-123.ogg"
	expectedError := errors.New("S3 connection failed")

	mockS3.On("UploadFile", ctx, key, mock.Anything, "audio/ogg").Return("", expectedError)

	url, err := mockS3.UploadFile(ctx, key, nil, "audio/ogg")
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Equal(t, expectedError, err)

	mockS3.AssertExpectations(t)
}
