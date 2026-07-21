package worker

import (
	"context"
	"testing"
	"time"
	"voxly/internal/stt"
	"voxly/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTranscriber is a stand-in stt.Transcriber for tests.
type MockTranscriber struct {
	mock.Mock
}

func (m *MockTranscriber) Transcribe(ctx context.Context, a stt.Audio) (*stt.Result, error) {
	args := m.Called(ctx, a)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*stt.Result), args.Error(1)
}

func (m *MockTranscriber) Name() string { return "mock" }

func TestMockTranscriber_Satisfies(t *testing.T) {
	var _ stt.Transcriber = (*MockTranscriber)(nil)
}

func TestTaskErrorTransition(t *testing.T) {
	task := &model.Task{
		ID:        "task-123",
		Status:    model.TaskStatusInProgress,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	task.SetError("boom")
	task.IncrementAttempts()

	assert.Equal(t, model.TaskStatusFailed, task.Status)
	assert.Equal(t, 1, task.Attempts)
	assert.NotNil(t, task.ErrorText)
	assert.Equal(t, "boom", *task.ErrorText)
}
