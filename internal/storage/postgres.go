package storage

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"voxly/pkg/logger"
	"voxly/pkg/model"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

type PostgresStorage struct {
	pool *pgxpool.Pool
}

// New PostgreSQL storage instance
func NewPostgresStorage(databaseURL string) (*PostgresStorage, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection established")

	// Run migrations
	if err := runMigrations(databaseURL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("Database migrations completed successfully")

	return &PostgresStorage{pool: pool}, nil
}

// Executing database migrations
func runMigrations(databaseURL string) error {
	// Get absolute path to migrations directory
	migrationsPath, err := filepath.Abs("migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	// Create file URL from path (works on both Windows and Unix)
	var migrationsURL string
	if runtime.GOOS == "windows" {
		// On Windows
		u := &url.URL{
			Scheme: "file",
			Path:   filepath.ToSlash(migrationsPath),
		}
		migrationsURL = u.String()
	} else {
		// On Unix
		migrationsURL = fmt.Sprintf("file://%s", migrationsPath)
	}

	logger.Info("Running migrations", zap.String("path", migrationsURL))

	// Create a standard database connection for migrations
	db := stdlib.OpenDB(*parseConfig(databaseURL))
	defer db.Close()

	// Create postgres driver instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		migrationsURL,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Run migrations up
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		logger.Info("No new migrations to apply")
	} else {
		logger.Info("Migrations applied successfully")
	}

	return nil
}

// Drops all tables and re-runs migrations (for development)
func ResetMigrations(databaseURL string) error {
	logger.Warn("Resetting database - this will drop all data!")

	// Get absolute path to migrations directory
	migrationsPath, err := filepath.Abs("migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	// Create file URL from path 
	var migrationsURL string
	if runtime.GOOS == "windows" {
		u := &url.URL{
			Scheme: "file",
			Path:   filepath.ToSlash(migrationsPath),
		}
		migrationsURL = u.String()
	} else {
		migrationsURL = fmt.Sprintf("file://%s", migrationsPath)
	}

	// Create a standard database connection for migrations
	db := stdlib.OpenDB(*parseConfig(databaseURL))
	defer db.Close()

	// Create postgres driver instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		migrationsURL,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Drop everything
	if err := m.Drop(); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	logger.Info("Database dropped successfully")

	// Run migrations up again
	if err := m.Up(); err != nil {
		return fmt.Errorf("failed to run migrations after reset: %w", err)
	}

	logger.Info("Database reset and migrations applied successfully")
	return nil
}

// Parses database URL into pgx config
func parseConfig(databaseURL string) *pgx.ConnConfig {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		logger.Fatal("Failed to parse database URL", zap.Error(err))
	}
	return config
}

// Closes the database connection pool
func (s *PostgresStorage) Close() {
	s.pool.Close()
}

// CreateTask inserts a new task into the database
func (s *PostgresStorage) CreateTask(ctx context.Context, task *model.Task) error {
	query := `
		INSERT INTO tasks (
			id, telegram_message_id, chat_id, file_id, status, 
			operation_id, attempts, error_text, meta, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	_, err := s.pool.Exec(ctx, query,
		task.ID,
		task.TelegramMessageID,
		task.ChatID,
		task.FileID,
		task.Status,
		task.OperationID,
		task.Attempts,
		task.ErrorText,
		task.Meta,
		task.CreatedAt,
		task.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

// GetTaskByID retrieves a task by its ID
func (s *PostgresStorage) GetTaskByID(ctx context.Context, id string) (*model.Task, error) {
	query := `
		SELECT id, telegram_message_id, chat_id, file_id, status,
		       operation_id, attempts, error_text, meta, created_at, updated_at
		FROM tasks
		WHERE id = $1`

	var task model.Task
	row := s.pool.QueryRow(ctx, query, id)

	err := row.Scan(
		&task.ID,
		&task.TelegramMessageID,
		&task.ChatID,
		&task.FileID,
		&task.Status,
		&task.OperationID,
		&task.Attempts,
		&task.ErrorText,
		&task.Meta,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return &task, nil
}

// UpdateTaskStatus updates the status of a task
func (s *PostgresStorage) UpdateTaskStatus(ctx context.Context, id string, status model.TaskStatus) error {
	query := `
		UPDATE tasks 
		SET status = $2, updated_at = NOW()
		WHERE id = $1`

	result, err := s.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("task not found")
	}

	return nil
}

// UpdateTask updates a full task
func (s *PostgresStorage) UpdateTask(ctx context.Context, task *model.Task) error {
	query := `
		UPDATE tasks 
		SET telegram_message_id = $2, chat_id = $3, file_id = $4, status = $5,
		    operation_id = $6, attempts = $7, error_text = $8, meta = $9, updated_at = $10
		WHERE id = $1`

	result, err := s.pool.Exec(ctx, query,
		task.ID,
		task.TelegramMessageID,
		task.ChatID,
		task.FileID,
		task.Status,
		task.OperationID,
		task.Attempts,
		task.ErrorText,
		task.Meta,
		task.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("task not found")
	}

	return nil
}

// GetQueuedTasks retrieves all tasks with queued status
func (s *PostgresStorage) GetQueuedTasks(ctx context.Context, limit int) ([]*model.Task, error) {
	query := `
		SELECT id, telegram_message_id, chat_id, file_id, status,
		       operation_id, attempts, error_text, meta, created_at, updated_at
		FROM tasks
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := s.pool.Query(ctx, query, model.TaskStatusQueued, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get queued tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		var task model.Task
		err := rows.Scan(
			&task.ID,
			&task.TelegramMessageID,
			&task.ChatID,
			&task.FileID,
			&task.Status,
			&task.OperationID,
			&task.Attempts,
			&task.ErrorText,
			&task.Meta,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate tasks: %w", err)
	}

	return tasks, nil
}

// CreateTranscript inserts a new transcript into the database
func (s *PostgresStorage) CreateTranscript(ctx context.Context, transcript *model.Transcript) error {
	query := `
		INSERT INTO transcripts (id, task_id, text, raw_response, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := s.pool.Exec(ctx, query,
		transcript.ID,
		transcript.TaskID,
		transcript.Text,
		transcript.RawResponse,
		transcript.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create transcript: %w", err)
	}

	return nil
}

// GetTranscriptByTaskID retrieves a transcript by task ID
func (s *PostgresStorage) GetTranscriptByTaskID(ctx context.Context, taskID string) (*model.Transcript, error) {
	query := `
		SELECT id, task_id, text, raw_response, created_at
		FROM transcripts
		WHERE task_id = $1`

	var transcript model.Transcript
	row := s.pool.QueryRow(ctx, query, taskID)

	err := row.Scan(
		&transcript.ID,
		&transcript.TaskID,
		&transcript.Text,
		&transcript.RawResponse,
		&transcript.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("transcript not found")
		}
		return nil, fmt.Errorf("failed to get transcript: %w", err)
	}

	return &transcript, nil
}
