package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aristath/orchestrator/internal/scheduler"
	_ "modernc.org/sqlite"
)

// ConversationTurn represents a single message in a task's conversation history.
type ConversationTurn struct {
	Role      string    // "user" or "assistant"
	Content   string
	Timestamp time.Time
}

// Store defines the persistence interface for tasks, sessions, and conversation history.
type Store interface {
	// Task DAG operations
	SaveTask(ctx context.Context, task *scheduler.Task) error
	GetTask(ctx context.Context, taskID string) (*scheduler.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID string, status scheduler.TaskStatus, result string, taskErr error) error
	ListTasks(ctx context.Context) ([]*scheduler.Task, error)

	// Session operations
	SaveSession(ctx context.Context, taskID, sessionID, backendType string) error
	GetSession(ctx context.Context, taskID string) (sessionID string, backendType string, err error)

	// Conversation history
	SaveMessage(ctx context.Context, taskID, role, content string) error
	GetHistory(ctx context.Context, taskID string) ([]ConversationTurn, error)

	// Lifecycle
	Close() error
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed store at the given path.
// Creates parent directories if needed. Enables WAL mode, foreign keys, and busy timeout.
func NewSQLiteStore(ctx context.Context, dbPath string) (*SQLiteStore, error) {
	// Create parent directories
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Open SQLite with connection string for WAL mode, foreign keys, busy timeout
	connStr := fmt.Sprintf("file:%s?_foreign_keys=ON&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Single-writer safety
	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db}

	// Initialize schema
	if err := store.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// NewMemoryStore creates an in-memory SQLite store for testing.
// Does not use WAL mode (not needed for in-memory databases).
func NewMemoryStore(ctx context.Context) (*SQLiteStore, error) {
	connStr := ":memory:?_foreign_keys=ON"
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open memory database: %w", err)
	}

	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db}

	// Initialize schema
	if err := store.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
