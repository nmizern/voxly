package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRedisCache struct {
	mock.Mock
	data map[string]interface{}
}

func NewMockRedisCache() *MockRedisCache {
	return &MockRedisCache{
		data: make(map[string]interface{}),
	}
}

func (m *MockRedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	args := m.Called(ctx, key, dest)
	return args.Error(0)
}

func (m *MockRedisCache) Set(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	if args.Error(0) == nil {
		m.data[key] = value
	}
	return args.Error(0)
}

func (m *MockRedisCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockRedisCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	if args.Error(0) == nil {
		delete(m.data, key)
	}
	return args.Error(0)
}

func (m *MockRedisCache) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockRedisCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestRedisCache_SetAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	mockCache := NewMockRedisCache()
	ctx := context.Background()

	type TestData struct {
		ID   string
		Name string
	}

	testData := TestData{ID: "123", Name: "test"}
	key := "test:key"

	mockCache.On("Set", ctx, key, testData).Return(nil)
	mockCache.On("Get", ctx, key, mock.AnythingOfType("*cache.TestData")).Return(nil)

	err := mockCache.Set(ctx, key, testData)
	assert.NoError(t, err)

	var retrieved TestData
	err = mockCache.Get(ctx, key, &retrieved)
	assert.NoError(t, err)

	mockCache.AssertExpectations(t)
}

func TestRedisCache_Delete(t *testing.T) {
	mockCache := NewMockRedisCache()
	ctx := context.Background()
	key := "test:key"

	mockCache.data[key] = "value"

	mockCache.On("Delete", ctx, key).Return(nil)

	err := mockCache.Delete(ctx, key)
	assert.NoError(t, err)
	assert.NotContains(t, mockCache.data, key)

	mockCache.AssertExpectations(t)
}

func TestRedisCache_Exists(t *testing.T) {
	mockCache := NewMockRedisCache()
	ctx := context.Background()
	key := "test:key"

	mockCache.On("Exists", ctx, key).Return(false, nil)

	exists, err := mockCache.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	mockCache.AssertExpectations(t)
}

func TestCacheKey_String(t *testing.T) {
	key := CacheKey{Prefix: "task", ID: "123"}
	assert.Equal(t, "task:123", key.String())
}

func TestTaskCacheKey(t *testing.T) {
	key := TaskCacheKey("task-123")
	assert.Equal(t, "task:task-123", key)
}

func TestTranscriptCacheKey(t *testing.T) {
	key := TranscriptCacheKey("task-456")
	assert.Equal(t, "transcript:task-456", key)
}

func TestChatActiveCacheKey(t *testing.T) {
	key := ChatActiveCacheKey(123456)
	assert.Equal(t, "chat:active:123456", key)
}
