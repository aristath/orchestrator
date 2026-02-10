package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/aristath/orchestrator/internal/scheduler"
)

// SaveTask saves or updates a task and its dependencies.
// Uses ON CONFLICT to make saves idempotent.
func (s *SQLiteStore) SaveTask(ctx context.Context, task *scheduler.Task) error {
	// Begin transaction with serializable isolation (BEGIN IMMEDIATE)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Convert error to string for storage
	errorStr := ""
	if task.Error != nil {
		errorStr = task.Error.Error()
	}

	// Convert WritesFiles slice to comma-separated string
	writesFiles := strings.Join(task.WritesFiles, ",")

	// Upsert task (insert or update on conflict)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tasks (id, name, agent_role, prompt, writes_files, status, failure_mode, result, error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			agent_role = excluded.agent_role,
			prompt = excluded.prompt,
			writes_files = excluded.writes_files,
			status = excluded.status,
			failure_mode = excluded.failure_mode,
			result = excluded.result,
			error = excluded.error,
			updated_at = CURRENT_TIMESTAMP
	`, task.ID, task.Name, task.AgentRole, task.Prompt, writesFiles, task.Status, task.FailureMode, task.Result, errorStr)
	if err != nil {
		return fmt.Errorf("failed to upsert task: %w", err)
	}

	// Delete existing dependencies for this task
	_, err = tx.ExecContext(ctx, `DELETE FROM task_dependencies WHERE task_id = ?`, task.ID)
	if err != nil {
		return fmt.Errorf("failed to delete old dependencies: %w", err)
	}

	// Insert new dependencies
	for _, depID := range task.DependsOn {
		// Check if dependency exists (enforces foreign key)
		var exists int
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM tasks WHERE id = ?`, depID).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Errorf("foreign key constraint failed: dependency task %s does not exist", depID)
		}
		if err != nil {
			return fmt.Errorf("failed to check dependency existence: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO task_dependencies (task_id, depends_on_id)
			VALUES (?, ?)
		`, task.ID, depID)
		if err != nil {
			return fmt.Errorf("failed to insert dependency %s -> %s: %w", task.ID, depID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTask retrieves a task by ID, including its dependencies.
func (s *SQLiteStore) GetTask(ctx context.Context, taskID string) (*scheduler.Task, error) {
	task := &scheduler.Task{}
	var errorStr string
	var writesFiles string

	// Load task fields
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, agent_role, prompt, writes_files, status, failure_mode, result, error
		FROM tasks
		WHERE id = ?
	`, taskID).Scan(&task.ID, &task.Name, &task.AgentRole, &task.Prompt, &writesFiles, &task.Status, &task.FailureMode, &task.Result, &errorStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query task: %w", err)
	}

	// Parse WritesFiles from comma-separated string
	if writesFiles != "" {
		task.WritesFiles = strings.Split(writesFiles, ",")
	}

	// Reconstruct error if present
	if errorStr != "" {
		task.Error = fmt.Errorf("%s", errorStr)
	}

	// Load dependencies
	rows, err := s.db.QueryContext(ctx, `
		SELECT depends_on_id
		FROM task_dependencies
		WHERE task_id = ?
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query dependencies: %w", err)
	}
	defer rows.Close()

	task.DependsOn = []string{}
	for rows.Next() {
		var depID string
		if err := rows.Scan(&depID); err != nil {
			return nil, fmt.Errorf("failed to scan dependency: %w", err)
		}
		task.DependsOn = append(task.DependsOn, depID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating dependencies: %w", err)
	}

	return task, nil
}

// UpdateTaskStatus updates the status, result, and error of a task.
func (s *SQLiteStore) UpdateTaskStatus(ctx context.Context, taskID string, status scheduler.TaskStatus, result string, taskErr error) error {
	// Begin transaction with serializable isolation (BEGIN IMMEDIATE)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Convert error to string
	errorStr := ""
	if taskErr != nil {
		errorStr = taskErr.Error()
	}

	// Update task status
	res, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, result = ?, error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, result, errorStr, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Check if task was found
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListTasks returns all tasks with their dependencies.
func (s *SQLiteStore) ListTasks(ctx context.Context) ([]*scheduler.Task, error) {
	// Query all tasks
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, agent_role, prompt, writes_files, status, failure_mode, result, error
		FROM tasks
		ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*scheduler.Task
	for rows.Next() {
		task := &scheduler.Task{}
		var errorStr string
		var writesFiles string

		err := rows.Scan(&task.ID, &task.Name, &task.AgentRole, &task.Prompt, &writesFiles, &task.Status, &task.FailureMode, &task.Result, &errorStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		// Parse WritesFiles from comma-separated string
		if writesFiles != "" {
			task.WritesFiles = strings.Split(writesFiles, ",")
		}

		// Reconstruct error if present
		if errorStr != "" {
			task.Error = fmt.Errorf("%s", errorStr)
		}

		// Load dependencies for this task
		depRows, err := s.db.QueryContext(ctx, `
			SELECT depends_on_id
			FROM task_dependencies
			WHERE task_id = ?
		`, task.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query dependencies for task %s: %w", task.ID, err)
		}

		task.DependsOn = []string{}
		for depRows.Next() {
			var depID string
			if err := depRows.Scan(&depID); err != nil {
				depRows.Close()
				return nil, fmt.Errorf("failed to scan dependency: %w", err)
			}
			task.DependsOn = append(task.DependsOn, depID)
		}
		depRows.Close()

		if err := depRows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating dependencies: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// SaveSession stores session information for a task.
func (s *SQLiteStore) SaveSession(ctx context.Context, taskID, sessionID, backendType string) error {

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (task_id, session_id, backend_type)
		VALUES (?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			session_id = excluded.session_id,
			backend_type = excluded.backend_type
	`, taskID, sessionID, backendType)

	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

// GetSession retrieves session information for a task.
func (s *SQLiteStore) GetSession(ctx context.Context, taskID string) (string, string, error) {

	var sessionID, backendType string
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, backend_type
		FROM sessions
		WHERE task_id = ?
	`, taskID).Scan(&sessionID, &backendType)

	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("session not found for task: %s", taskID)
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to query session: %w", err)
	}

	return sessionID, backendType, nil
}

// SaveMessage stores a conversation message for a task.
func (s *SQLiteStore) SaveMessage(ctx context.Context, taskID, role, content string) error {

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_history (task_id, role, content)
		VALUES (?, ?, ?)
	`, taskID, role, content)

	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	return nil
}

// GetHistory retrieves all conversation messages for a task.
func (s *SQLiteStore) GetHistory(ctx context.Context, taskID string) ([]ConversationTurn, error) {

	rows, err := s.db.QueryContext(ctx, `
		SELECT role, content, timestamp
		FROM conversation_history
		WHERE task_id = ?
		ORDER BY timestamp
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var history []ConversationTurn
	for rows.Next() {
		var turn ConversationTurn
		if err := rows.Scan(&turn.Role, &turn.Content, &turn.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		history = append(history, turn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating history: %w", err)
	}

	return history, nil
}
