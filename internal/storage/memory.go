package storage

import (
	"context"
	"fmt"
	"sync"
	"voxly/pkg/model"
)

// MemoryStore keeps tasks and transcripts in memory for lite deployments.
type MemoryStore struct {
	mu          sync.RWMutex
	tasks       map[string]*model.Task
	transcripts map[string]*model.Transcript
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks:       make(map[string]*model.Task),
		transcripts: make(map[string]*model.Transcript),
	}
}

func (s *MemoryStore) CreateTask(_ context.Context, task *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *task
	s.tasks[task.ID] = &cp
	return nil
}

func (s *MemoryStore) GetTaskByID(_ context.Context, id string) (*model.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	cp := *task
	return &cp, nil
}

func (s *MemoryStore) UpdateTask(_ context.Context, task *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[task.ID]; !ok {
		return fmt.Errorf("task not found")
	}
	cp := *task
	s.tasks[task.ID] = &cp
	return nil
}

func (s *MemoryStore) CreateTranscript(_ context.Context, transcript *model.Transcript) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *transcript
	s.transcripts[transcript.TaskID] = &cp
	return nil
}

func (s *MemoryStore) Close() error { return nil }
