
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Table tasks: stores a task for one voice message
CREATE TABLE IF NOT EXISTS tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  telegram_message_id BIGINT NOT NULL,
  chat_id BIGINT NOT NULL,
  file_id TEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'queued', -- queued, in_progress, done, failed
  operation_id TEXT,                              -- id Yandex SpeechKit
  attempts INT NOT NULL DEFAULT 0,                -- number of processing attempts
  error_text TEXT,                                -- error text (if any)
  meta JSONB DEFAULT '{}'::jsonb,                 -- additional data (optional)
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unique index to prevent duplication (same message)
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_chat_message ON tasks (chat_id, telegram_message_id);

-- Indexes for searching by operation_id and status
CREATE INDEX IF NOT EXISTS idx_tasks_operation_id ON tasks (operation_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks (status);

-- Trigger for updating updated_at
CREATE OR REPLACE FUNCTION trg_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_tasks_updated_at ON tasks;
CREATE TRIGGER trg_tasks_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION trg_set_timestamp();

-- Table transcripts: stores the final recognized text and raw JSON
CREATE TABLE IF NOT EXISTS transcripts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  text TEXT NOT NULL,
  raw_response JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for quick search of transcripts by task
CREATE INDEX IF NOT EXISTS idx_transcripts_task_id ON transcripts (task_id);
