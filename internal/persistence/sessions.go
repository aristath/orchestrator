package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SaveSession stores session information for a task.
// Uses ON CONFLICT to upsert - handles both first-save and resume scenarios.
func (s *SQLiteStore) SaveSession(ctx context.Context, taskID, sessionID, backendType string) error {
	// Create 5-second timeout context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Begin transaction with serializable isolation (BEGIN IMMEDIATE)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Upsert session (insert or update on conflict)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (task_id, session_id, backend_type)
		VALUES (?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			session_id = excluded.session_id,
			backend_type = excluded.backend_type
	`, taskID, sessionID, backendType)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetSession retrieves session information for a task.
// Returns a wrapped sql.ErrNoRows if no session exists for the task.
func (s *SQLiteStore) GetSession(ctx context.Context, taskID string) (string, string, error) {
	// Create 5-second timeout context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var sessionID, backendType string
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, backend_type
		FROM sessions
		WHERE task_id = ?
	`, taskID).Scan(&sessionID, &backendType)

	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("no session found for task %q: %w", taskID, err)
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to query session: %w", err)
	}

	return sessionID, backendType, nil
}

// SaveMessage stores a conversation message for a task.
// Messages are append-only (no upsert needed).
func (s *SQLiteStore) SaveMessage(ctx context.Context, taskID, role, content string) error {
	// Create 5-second timeout context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Begin transaction with serializable isolation (BEGIN IMMEDIATE)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert message (append-only)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO conversation_history (task_id, role, content)
		VALUES (?, ?, ?)
	`, taskID, role, content)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetHistory retrieves all conversation messages for a task in chronological order.
// Returns empty slice (not nil) if no history exists.
func (s *SQLiteStore) GetHistory(ctx context.Context, taskID string) ([]ConversationTurn, error) {
	// Create 5-second timeout context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Double sort: timestamp ASC, id ASC ensures correct order even with same-second timestamps
	rows, err := s.db.QueryContext(ctx, `
		SELECT role, content, timestamp
		FROM conversation_history
		WHERE task_id = ?
		ORDER BY timestamp ASC, id ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	// Return empty slice (not nil) if no history
	history := []ConversationTurn{}
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
