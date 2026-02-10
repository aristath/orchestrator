package persistence

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aristath/orchestrator/internal/scheduler"
)

// testStore creates an in-memory store for testing and registers cleanup.
func testStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewMemoryStore(context.Background())
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestSaveAndGetTask(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create a task with dependencies
	task := &scheduler.Task{
		ID:          "task-1",
		Name:        "Test Task",
		AgentRole:   "coder",
		Prompt:      "Write code",
		WritesFiles: []string{"file1.go", "file2.go"},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
		DependsOn:   []string{"dep-1", "dep-2"},
		Result:      "",
		Error:       nil,
	}

	// Save dependencies first (to satisfy foreign key constraints)
	dep1 := &scheduler.Task{
		ID:          "dep-1",
		Name:        "Dependency 1",
		AgentRole:   "setup",
		Prompt:      "Setup",
		Status:      scheduler.TaskCompleted,
		FailureMode: scheduler.FailHard,
	}
	dep2 := &scheduler.Task{
		ID:          "dep-2",
		Name:        "Dependency 2",
		AgentRole:   "setup",
		Prompt:      "Setup 2",
		Status:      scheduler.TaskCompleted,
		FailureMode: scheduler.FailHard,
	}

	if err := store.SaveTask(ctx, dep1); err != nil {
		t.Fatalf("failed to save dep1: %v", err)
	}
	if err := store.SaveTask(ctx, dep2); err != nil {
		t.Fatalf("failed to save dep2: %v", err)
	}

	// Save the task
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Retrieve the task
	retrieved, err := store.GetTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	// Verify all fields match
	if retrieved.ID != task.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, task.ID)
	}
	if retrieved.Name != task.Name {
		t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, task.Name)
	}
	if retrieved.AgentRole != task.AgentRole {
		t.Errorf("AgentRole mismatch: got %s, want %s", retrieved.AgentRole, task.AgentRole)
	}
	if retrieved.Prompt != task.Prompt {
		t.Errorf("Prompt mismatch: got %s, want %s", retrieved.Prompt, task.Prompt)
	}
	if retrieved.Status != task.Status {
		t.Errorf("Status mismatch: got %v, want %v", retrieved.Status, task.Status)
	}
	if retrieved.FailureMode != task.FailureMode {
		t.Errorf("FailureMode mismatch: got %v, want %v", retrieved.FailureMode, task.FailureMode)
	}
	if len(retrieved.WritesFiles) != len(task.WritesFiles) {
		t.Errorf("WritesFiles length mismatch: got %d, want %d", len(retrieved.WritesFiles), len(task.WritesFiles))
	}
	for i, f := range task.WritesFiles {
		if retrieved.WritesFiles[i] != f {
			t.Errorf("WritesFiles[%d] mismatch: got %s, want %s", i, retrieved.WritesFiles[i], f)
		}
	}
	if len(retrieved.DependsOn) != len(task.DependsOn) {
		t.Errorf("DependsOn length mismatch: got %d, want %d", len(retrieved.DependsOn), len(task.DependsOn))
	}
	for i, dep := range task.DependsOn {
		if retrieved.DependsOn[i] != dep {
			t.Errorf("DependsOn[%d] mismatch: got %s, want %s", i, retrieved.DependsOn[i], dep)
		}
	}
}

func TestSaveTaskIdempotent(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	task := &scheduler.Task{
		ID:          "task-idempotent",
		Name:        "Idempotent Task",
		AgentRole:   "coder",
		Prompt:      "Test idempotency",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	// Save task initially
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Update status to Completed
	task.Status = scheduler.TaskCompleted
	task.Result = "Success"

	// Save again (should update, not error)
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task second time: %v", err)
	}

	// Retrieve and verify status is Completed
	retrieved, err := store.GetTask(ctx, "task-idempotent")
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if retrieved.Status != scheduler.TaskCompleted {
		t.Errorf("Status should be Completed after update, got %v", retrieved.Status)
	}
	if retrieved.Result != "Success" {
		t.Errorf("Result mismatch: got %s, want Success", retrieved.Result)
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	task := &scheduler.Task{
		ID:          "task-status",
		Name:        "Status Task",
		AgentRole:   "coder",
		Prompt:      "Test status updates",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	// Save task as Pending
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Update to Running
	if err := store.UpdateTaskStatus(ctx, "task-status", scheduler.TaskRunning, "", nil); err != nil {
		t.Fatalf("failed to update to Running: %v", err)
	}

	retrieved, err := store.GetTask(ctx, "task-status")
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if retrieved.Status != scheduler.TaskRunning {
		t.Errorf("Status should be Running, got %v", retrieved.Status)
	}

	// Update to Completed with result
	if err := store.UpdateTaskStatus(ctx, "task-status", scheduler.TaskCompleted, "Task completed successfully", nil); err != nil {
		t.Fatalf("failed to update to Completed: %v", err)
	}

	retrieved, err = store.GetTask(ctx, "task-status")
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if retrieved.Status != scheduler.TaskCompleted {
		t.Errorf("Status should be Completed, got %v", retrieved.Status)
	}
	if retrieved.Result != "Task completed successfully" {
		t.Errorf("Result mismatch: got %s, want 'Task completed successfully'", retrieved.Result)
	}
}

func TestUpdateTaskStatusNotFound(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Try to update a non-existent task
	err := store.UpdateTaskStatus(ctx, "nonexistent", scheduler.TaskCompleted, "result", nil)
	if err == nil {
		t.Fatal("expected error when updating non-existent task, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestListTasks(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create 3 tasks with dependencies
	task1 := &scheduler.Task{
		ID:          "list-task-1",
		Name:        "Task 1",
		AgentRole:   "setup",
		Prompt:      "Setup task",
		Status:      scheduler.TaskCompleted,
		FailureMode: scheduler.FailHard,
	}

	task2 := &scheduler.Task{
		ID:          "list-task-2",
		Name:        "Task 2",
		AgentRole:   "coder",
		Prompt:      "Code task",
		Status:      scheduler.TaskRunning,
		FailureMode: scheduler.FailHard,
		DependsOn:   []string{"list-task-1"},
	}

	task3 := &scheduler.Task{
		ID:          "list-task-3",
		Name:        "Task 3",
		AgentRole:   "reviewer",
		Prompt:      "Review task",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailSoft,
		DependsOn:   []string{"list-task-1", "list-task-2"},
	}

	// Save all tasks
	if err := store.SaveTask(ctx, task1); err != nil {
		t.Fatalf("failed to save task1: %v", err)
	}
	if err := store.SaveTask(ctx, task2); err != nil {
		t.Fatalf("failed to save task2: %v", err)
	}
	if err := store.SaveTask(ctx, task3); err != nil {
		t.Fatalf("failed to save task3: %v", err)
	}

	// List all tasks
	tasks, err := store.ListTasks(ctx)
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}

	// Verify count
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Verify fields for each task
	taskMap := make(map[string]*scheduler.Task)
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	if taskMap["list-task-1"].Name != "Task 1" {
		t.Errorf("Task 1 name mismatch")
	}
	if len(taskMap["list-task-2"].DependsOn) != 1 {
		t.Errorf("Task 2 should have 1 dependency, got %d", len(taskMap["list-task-2"].DependsOn))
	}
	if len(taskMap["list-task-3"].DependsOn) != 2 {
		t.Errorf("Task 3 should have 2 dependencies, got %d", len(taskMap["list-task-3"].DependsOn))
	}
}

func TestForeignKeyEnforced(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Try to save a task with a non-existent dependency
	task := &scheduler.Task{
		ID:          "fk-task",
		Name:        "FK Task",
		AgentRole:   "coder",
		Prompt:      "Test FK",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
		DependsOn:   []string{"nonexistent-dep"},
	}

	err := store.SaveTask(ctx, task)
	if err == nil {
		t.Fatal("expected error when inserting dependency on non-existent task, got nil")
	}

	// The error should mention foreign key or constraint
	errStr := err.Error()
	if !strings.Contains(errStr, "foreign key") && !strings.Contains(errStr, "constraint") && !strings.Contains(errStr, "FOREIGN KEY") {
		t.Logf("Warning: error doesn't explicitly mention foreign key: %v", err)
		// Still pass test if we got an error (foreign keys are working)
	}
}

func TestTaskErrorPersistence(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	task := &scheduler.Task{
		ID:          "error-task",
		Name:        "Error Task",
		AgentRole:   "coder",
		Prompt:      "Test error persistence",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	// Save task initially
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Update with error
	testError := fmt.Errorf("task failed: file not found")
	if err := store.UpdateTaskStatus(ctx, "error-task", scheduler.TaskFailed, "", testError); err != nil {
		t.Fatalf("failed to update task with error: %v", err)
	}

	// Retrieve and verify error persisted
	retrieved, err := store.GetTask(ctx, "error-task")
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if retrieved.Error == nil {
		t.Fatal("expected error to be persisted, got nil")
	}
	if retrieved.Error.Error() != testError.Error() {
		t.Errorf("Error mismatch: got %v, want %v", retrieved.Error, testError)
	}
	if retrieved.Status != scheduler.TaskFailed {
		t.Errorf("Status should be Failed, got %v", retrieved.Status)
	}
}

// Session tests

func TestSaveAndGetSession(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create a task first (required by foreign key)
	task := &scheduler.Task{
		ID:          "session-task-1",
		Name:        "Session Task",
		AgentRole:   "coder",
		Prompt:      "Test session",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save session
	if err := store.SaveSession(ctx, "session-task-1", "session-abc-123", "claude"); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	// Get session
	sessionID, backendType, err := store.GetSession(ctx, "session-task-1")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	// Verify session data
	if sessionID != "session-abc-123" {
		t.Errorf("SessionID mismatch: got %s, want session-abc-123", sessionID)
	}
	if backendType != "claude" {
		t.Errorf("BackendType mismatch: got %s, want claude", backendType)
	}
}

func TestSaveSessionUpsert(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create a task first
	task := &scheduler.Task{
		ID:          "upsert-task",
		Name:        "Upsert Task",
		AgentRole:   "coder",
		Prompt:      "Test upsert",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save session with sessionID "abc"
	if err := store.SaveSession(ctx, "upsert-task", "abc", "claude"); err != nil {
		t.Fatalf("failed to save initial session: %v", err)
	}

	// Save again with sessionID "def" for same task (should update)
	if err := store.SaveSession(ctx, "upsert-task", "def", "codex"); err != nil {
		t.Fatalf("failed to save updated session: %v", err)
	}

	// Get session and verify it was updated
	sessionID, backendType, err := store.GetSession(ctx, "upsert-task")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if sessionID != "def" {
		t.Errorf("SessionID should be 'def' after upsert, got %s", sessionID)
	}
	if backendType != "codex" {
		t.Errorf("BackendType should be 'codex' after upsert, got %s", backendType)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Try to get session for non-existent task
	_, _, err := store.GetSession(ctx, "nonexistent-task")
	if err == nil {
		t.Fatal("expected error when getting session for non-existent task, got nil")
	}

	// Verify error wraps sql.ErrNoRows
	if !strings.Contains(err.Error(), "no session found") {
		t.Errorf("expected 'no session found' in error, got: %v", err)
	}
}

func TestSessionBackendTypes(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create 3 tasks with different backend types
	backends := []struct {
		taskID      string
		sessionID   string
		backendType string
	}{
		{"claude-task", "session-1", "claude"},
		{"codex-task", "session-2", "codex"},
		{"goose-task", "session-3", "goose"},
	}

	for _, b := range backends {
		task := &scheduler.Task{
			ID:          b.taskID,
			Name:        "Backend Task",
			AgentRole:   "coder",
			Prompt:      "Test backend types",
			Status:      scheduler.TaskPending,
			FailureMode: scheduler.FailHard,
		}
		if err := store.SaveTask(ctx, task); err != nil {
			t.Fatalf("failed to save task %s: %v", b.taskID, err)
		}

		if err := store.SaveSession(ctx, b.taskID, b.sessionID, b.backendType); err != nil {
			t.Fatalf("failed to save session for %s: %v", b.taskID, err)
		}
	}

	// Verify each backend type is correctly retrieved
	for _, b := range backends {
		sessionID, backendType, err := store.GetSession(ctx, b.taskID)
		if err != nil {
			t.Fatalf("failed to get session for %s: %v", b.taskID, err)
		}

		if sessionID != b.sessionID {
			t.Errorf("SessionID mismatch for %s: got %s, want %s", b.taskID, sessionID, b.sessionID)
		}
		if backendType != b.backendType {
			t.Errorf("BackendType mismatch for %s: got %s, want %s", b.taskID, backendType, b.backendType)
		}
	}
}

// Conversation history tests

func TestSaveAndGetHistory(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create a task first
	task := &scheduler.Task{
		ID:          "history-task",
		Name:        "History Task",
		AgentRole:   "coder",
		Prompt:      "Test history",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save 3 messages
	messages := []struct {
		role    string
		content string
	}{
		{"user", "Hello, can you help?"},
		{"assistant", "Sure, I can help!"},
		{"user", "Great, thanks!"},
	}

	for _, msg := range messages {
		if err := store.SaveMessage(ctx, "history-task", msg.role, msg.content); err != nil {
			t.Fatalf("failed to save message: %v", err)
		}
	}

	// Get history
	history, err := store.GetHistory(ctx, "history-task")
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	// Verify 3 turns in correct order
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}

	for i, msg := range messages {
		if history[i].Role != msg.role {
			t.Errorf("Message %d role mismatch: got %s, want %s", i, history[i].Role, msg.role)
		}
		if history[i].Content != msg.content {
			t.Errorf("Message %d content mismatch: got %s, want %s", i, history[i].Content, msg.content)
		}
	}
}

func TestGetHistoryEmpty(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create a task with no messages
	task := &scheduler.Task{
		ID:          "empty-history-task",
		Name:        "Empty History Task",
		AgentRole:   "coder",
		Prompt:      "Test empty history",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Get history (should be empty, not nil)
	history, err := store.GetHistory(ctx, "empty-history-task")
	if err != nil {
		t.Fatalf("failed to get empty history: %v", err)
	}

	// Verify empty slice (not nil) and no error
	if history == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(history) != 0 {
		t.Errorf("expected 0 messages, got %d", len(history))
	}
}

func TestGetHistoryOrdering(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create a task first
	task := &scheduler.Task{
		ID:          "ordering-task",
		Name:        "Ordering Task",
		AgentRole:   "coder",
		Prompt:      "Test ordering",
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save 5 messages rapidly (tests id-based tiebreaker for same-timestamp entries)
	messages := []string{"msg1", "msg2", "msg3", "msg4", "msg5"}
	for _, content := range messages {
		if err := store.SaveMessage(ctx, "ordering-task", "user", content); err != nil {
			t.Fatalf("failed to save message: %v", err)
		}
	}

	// Get history
	history, err := store.GetHistory(ctx, "ordering-task")
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	// Verify order matches insertion order
	if len(history) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(history))
	}

	for i, content := range messages {
		if history[i].Content != content {
			t.Errorf("Message %d out of order: got %s, want %s", i, history[i].Content, content)
		}
	}
}

func TestConversationHistoryForeignKey(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Verify the task doesn't exist first
	_, err := store.GetTask(ctx, "nonexistent-task")
	if err == nil {
		t.Fatal("nonexistent-task should not exist")
	}

	// Attempt to save message for non-existent task_id
	// This should fail due to foreign key constraint
	err = store.SaveMessage(ctx, "nonexistent-task", "user", "Hello")

	// Verify we get an error (foreign key constraint violation)
	if err == nil {
		t.Fatal("expected error when saving message for non-existent task, got nil")
	}

	// Log the error for debugging
	t.Logf("Got expected error: %v", err)
}
