

DROP INDEX IF EXISTS idx_transcripts_task_id;
DROP TABLE IF EXISTS transcripts;

DROP TRIGGER IF EXISTS trg_tasks_updated_at ON tasks;
DROP FUNCTION IF EXISTS trg_set_timestamp();

DROP INDEX IF EXISTS idx_tasks_operation_id;
DROP INDEX IF EXISTS idx_tasks_status;
DROP INDEX IF EXISTS idx_tasks_chat_message;
DROP TABLE IF EXISTS tasks;


-- DROP EXTENSION IF EXISTS "pgcrypto";
