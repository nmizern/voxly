package queue

import (
	"encoding/json"
	"errors"
	"sync"
	"voxly/pkg/logger"

	"go.uber.org/zap"
)

// MemoryQueue is an in process queue for lite deployments, no broker needed.
type MemoryQueue struct {
	ch   chan []byte
	done chan struct{}
	once sync.Once
}

func NewMemoryQueue(buffer int) *MemoryQueue {
	if buffer <= 0 {
		buffer = 256
	}
	return &MemoryQueue{
		ch:   make(chan []byte, buffer),
		done: make(chan struct{}),
	}
}

func (m *MemoryQueue) PublishTask(task *VoiceTask) error {
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}
	select {
	case m.ch <- body:
		return nil
	case <-m.done:
		return errors.New("queue is closed")
	}
}

func (m *MemoryQueue) Consume(_ string, handler func([]byte) error) error {
	for {
		select {
		case body := <-m.ch:
			if err := handler(body); err != nil {
				logger.Error("Failed to handle task", zap.Error(err))
			}
		case <-m.done:
			return nil
		}
	}
}

func (m *MemoryQueue) Close() error {
	m.once.Do(func() { close(m.done) })
	return nil
}
