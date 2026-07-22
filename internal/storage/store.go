package storage

import (
	"context"
	"voxly/pkg/model"
)

type Store interface {
	CreateTask(ctx context.Context, task *model.Task) error
	GetTaskByID(ctx context.Context, id string) (*model.Task, error)
	UpdateTask(ctx context.Context, task *model.Task) error
	CreateTranscript(ctx context.Context, transcript *model.Transcript) error
	Close() error
}
