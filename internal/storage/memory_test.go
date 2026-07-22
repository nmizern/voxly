package storage

import (
	"context"
	"testing"
	"time"
	"voxly/pkg/model"
)

var (
	_ Store = (*MemoryStore)(nil)
	_ Store = (*PostgresStorage)(nil)
)

func TestMemoryStore_TaskLifecycle(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	task := &model.Task{ID: "t1", Status: model.TaskStatusQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetTaskByID(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.TaskStatusQueued {
		t.Errorf("status = %v", got.Status)
	}

	// Mutating a returned copy must not leak into the store.
	got.Status = model.TaskStatusDone
	again, _ := s.GetTaskByID(ctx, "t1")
	if again.Status != model.TaskStatusQueued {
		t.Error("store was mutated through a returned copy")
	}

	task.Status = model.TaskStatusDone
	if err := s.UpdateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	upd, _ := s.GetTaskByID(ctx, "t1")
	if upd.Status != model.TaskStatusDone {
		t.Errorf("update not applied: %v", upd.Status)
	}
}

func TestMemoryStore_MissingTask(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.GetTaskByID(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for missing task")
	}
}
