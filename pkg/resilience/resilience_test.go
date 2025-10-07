package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_Closed(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	err := cb.Execute(func() error {
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	testErr := errors.New("test error")

	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return testErr
		})
		assert.Error(t, err)
	}

	assert.Equal(t, StateOpen, cb.GetState())

	err := cb.Execute(func() error {
		return nil
	})

	assert.Equal(t, ErrCircuitOpen, err)
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	testErr := errors.New("test error")

	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	assert.Equal(t, StateOpen, cb.GetState())

	time.Sleep(150 * time.Millisecond)

	err := cb.Execute(func() error {
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2, 5*time.Second)

	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	assert.Equal(t, StateOpen, cb.GetState())

	cb.Reset()

	assert.Equal(t, StateClosed, cb.GetState())
}

func TestRetryWithExponentialBackoff_Success(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxAttempts = 3

	attempts := 0
	err := RetryWithExponentialBackoff(ctx, config, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestRetryWithExponentialBackoff_MaxAttemptsReached(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.InitialInterval = 10 * time.Millisecond

	attempts := 0
	testErr := errors.New("persistent error")

	err := RetryWithExponentialBackoff(ctx, config, func() error {
		attempts++
		return testErr
	})

	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryWithExponentialBackoff_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultRetryConfig()
	config.MaxAttempts = 10
	config.InitialInterval = 100 * time.Millisecond

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RetryWithExponentialBackoff(ctx, config, func() error {
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)

	assert.True(t, rl.Allow())
	assert.True(t, rl.Allow())
	assert.False(t, rl.Allow())

	time.Sleep(150 * time.Millisecond)

	assert.True(t, rl.Allow())
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(1, 100*time.Millisecond)
	ctx := context.Background()

	rl.Allow()

	start := time.Now()
	err := rl.Wait(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, duration >= 100*time.Millisecond)
}

func TestRateLimiter_WaitWithTimeout(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	rl.Allow()

	err := rl.Wait(ctx)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}
