package persistence

import (
	"context"
)

// initSchema creates all required tables if they don't exist.
func (s *SQLiteStore) initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		agent_role TEXT NOT NULL,
		prompt TEXT NOT NULL,
		writes_files TEXT,
		status INTEGER NOT NULL,
		failure_mode INTEGER NOT NULL,
		result TEXT,
		error TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS task_dependencies (
		task_id TEXT NOT NULL,
		depends_on_id TEXT NOT NULL,
		PRIMARY KEY (task_id, depends_on_id),
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
		FOREIGN KEY (depends_on_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_task_dependencies_task_id ON task_dependencies(task_id);

	CREATE TABLE IF NOT EXISTS sessions (
		task_id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		backend_type TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS conversation_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_conversation_history_task_timestamp
		ON conversation_history(task_id, timestamp);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}
