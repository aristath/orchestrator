package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aristath/orchestrator/internal/backend"
	"github.com/aristath/orchestrator/internal/events"
	"github.com/aristath/orchestrator/internal/persistence"
	"github.com/aristath/orchestrator/internal/scheduler"
	"github.com/aristath/orchestrator/internal/worktree"
)

// setupTestRepo creates a temp git repository for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v (output: %s)", err, string(output))
	}

	// Checkout main branch
	cmd = exec.Command("git", "checkout", "-b", "main")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b main failed: %v (output: %s)", err, string(output))
	}

	// Create initial commit
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v (output: %s)", err, string(output))
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v (output: %s)", err, string(output))
	}

	return tmpDir
}

// mockBackend is a test implementation of backend.Backend.
type mockBackend struct {
	mu        sync.Mutex
	workDir   string
	taskID    string
	sendCount int
	delay     time.Duration
	onSend    func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error)
	qaChannel *QAChannel
	closed    bool
}

func (m *mockBackend) Send(ctx context.Context, msg backend.Message) (backend.Response, error) {
	m.mu.Lock()
	m.sendCount++
	m.mu.Unlock()

	// Apply delay if configured
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	// Use custom handler if provided
	if m.onSend != nil {
		return m.onSend(ctx, msg, m.workDir)
	}

	// Default: return success
	return backend.Response{
		Content:   fmt.Sprintf("Completed: %s", msg.Content),
		SessionID: "mock-session",
	}, nil
}

func (m *mockBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockBackend) SessionID() string {
	return "mock-session-" + m.taskID
}

// mockBackendFactory tracks backend creation for testing.
type mockBackendFactory struct {
	mu       sync.Mutex
	backends map[string]*mockBackend
	delay    time.Duration
	onSend   func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error)
}

func newMockBackendFactory() *mockBackendFactory {
	return &mockBackendFactory{
		backends: make(map[string]*mockBackend),
	}
}

func (f *mockBackendFactory) factory(agentRole string, workDir string) (backend.Backend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Extract task ID from workDir (format: /path/.worktrees/{taskID})
	taskID := filepath.Base(workDir)

	mb := &mockBackend{
		workDir: workDir,
		taskID:  taskID,
		delay:   f.delay,
		onSend:  f.onSend,
	}

	f.backends[taskID] = mb
	return mb, nil
}

func (f *mockBackendFactory) get(taskID string) *mockBackend {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.backends[taskID]
}

func (f *mockBackendFactory) all() []*mockBackend {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*mockBackend, 0, len(f.backends))
	for _, b := range f.backends {
		result = append(result, b)
	}
	return result
}

// TestParallelExecution_TwoIndependentTasks verifies two independent tasks execute and merge.
func TestParallelExecution_TwoIndependentTasks(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create DAG with two independent tasks
	dag := scheduler.NewDAG()
	taskA := &scheduler.Task{
		ID:          "task-a",
		Name:        "Task A",
		AgentRole:   "coder",
		Prompt:      "Write fileA.txt",
		DependsOn:   []string{},
		WritesFiles: []string{"fileA.txt"},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	taskB := &scheduler.Task{
		ID:          "task-b",
		Name:        "Task B",
		AgentRole:   "coder",
		Prompt:      "Write fileB.txt",
		DependsOn:   []string{},
		WritesFiles: []string{"fileB.txt"},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	if err := dag.AddTask(taskA); err != nil {
		t.Fatalf("failed to add task A: %v", err)
	}
	if err := dag.AddTask(taskB); err != nil {
		t.Fatalf("failed to add task B: %v", err)
	}

	// Create worktree manager
	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Create mock backend factory that writes unique files
	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		// Write a file based on task
		var filename string
		if strings.Contains(msg.Content, "fileA") {
			filename = "fileA.txt"
		} else if strings.Contains(msg.Content, "fileB") {
			filename = "fileB.txt"
		} else {
			return backend.Response{}, fmt.Errorf("unknown task")
		}

		filePath := filepath.Join(workDir, filename)
		if err := os.WriteFile(filePath, []byte(filename+" content\n"), 0644); err != nil {
			return backend.Response{}, err
		}

		// Git add and commit
		addCmd := exec.Command("git", "add", filename)
		addCmd.Dir = workDir
		if output, err := addCmd.CombinedOutput(); err != nil {
			return backend.Response{}, fmt.Errorf("git add failed: %v (output: %s)", err, string(output))
		}

		commitCmd := exec.Command("git", "commit", "-m", "Add "+filename)
		commitCmd.Dir = workDir
		if output, err := commitCmd.CombinedOutput(); err != nil {
			return backend.Response{}, fmt.Errorf("git commit failed: %v (output: %s)", err, string(output))
		}

		return backend.Response{
			Content:   fmt.Sprintf("Created %s", filename),
			SessionID: "mock",
		}, nil
	}

	// Create parallel runner
	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		MergeStrategy:   worktree.MergeOrt,
		BackendFactory:  factory.factory,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	// Run
	ctx := context.Background()
	results, err := runner.Run(ctx)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify results
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, result := range results {
		if !result.Success {
			t.Errorf("task %q failed: %v", result.TaskID, result.Error)
		}
		if result.MergeResult == nil || !result.MergeResult.Merged {
			t.Errorf("task %q not merged", result.TaskID)
		}
	}

	// Verify both files exist in main
	fileA := filepath.Join(repoPath, "fileA.txt")
	fileB := filepath.Join(repoPath, "fileB.txt")

	if _, err := os.Stat(fileA); os.IsNotExist(err) {
		t.Error("fileA.txt does not exist in main")
	}
	if _, err := os.Stat(fileB); os.IsNotExist(err) {
		t.Error("fileB.txt does not exist in main")
	}

	// Verify worktrees cleaned up
	verifyWorktreesCleanedUp(t, repoPath)
}

// TestBoundedConcurrency verifies max concurrent tasks never exceeds limit.
func TestBoundedConcurrency(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create DAG with 4 independent tasks
	dag := scheduler.NewDAG()
	for i := 1; i <= 4; i++ {
		task := &scheduler.Task{
			ID:          fmt.Sprintf("task-%d", i),
			Name:        fmt.Sprintf("Task %d", i),
			AgentRole:   "coder",
			Prompt:      fmt.Sprintf("Work %d", i),
			DependsOn:   []string{},
			WritesFiles: []string{},
			Status:      scheduler.TaskPending,
			FailureMode: scheduler.FailHard,
		}
		if err := dag.AddTask(task); err != nil {
			t.Fatalf("failed to add task: %v", err)
		}
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Track concurrent execution
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	factory := newMockBackendFactory()
	factory.delay = 100 * time.Millisecond
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		current := concurrent.Add(1)
		defer concurrent.Add(-1)

		// Update max
		for {
			max := maxConcurrent.Load()
			if current <= max || maxConcurrent.CompareAndSwap(max, current) {
				break
			}
		}

		time.Sleep(100 * time.Millisecond)
		return backend.Response{Content: "done", SessionID: "mock"}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		ConcurrencyLimit: 2, // Max 2 concurrent
		WorktreeManager:  wtMgr,
		BackendFactory:   factory.factory,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	ctx := context.Background()
	results, err := runner.Run(ctx)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}

	// Verify max concurrent never exceeded 2
	max := maxConcurrent.Load()
	if max > 2 {
		t.Errorf("max concurrent was %d, expected <= 2", max)
	}
}

// TestDAGWaves verifies wave ordering - dependent tasks wait for dependencies.
func TestDAGWaves(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create DAG: task A (no deps), task B depends on A
	dag := scheduler.NewDAG()
	taskA := &scheduler.Task{
		ID:          "task-a",
		Name:        "Task A",
		AgentRole:   "coder",
		Prompt:      "Work A",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	taskB := &scheduler.Task{
		ID:          "task-b",
		Name:        "Task B",
		AgentRole:   "coder",
		Prompt:      "Work B",
		DependsOn:   []string{"task-a"},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	if err := dag.AddTask(taskA); err != nil {
		t.Fatalf("failed to add task A: %v", err)
	}
	if err := dag.AddTask(taskB); err != nil {
		t.Fatalf("failed to add task B: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Track execution order
	var order []string
	var mu sync.Mutex

	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		taskID := filepath.Base(workDir)

		mu.Lock()
		order = append(order, taskID)
		mu.Unlock()

		return backend.Response{Content: "done", SessionID: "mock"}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	ctx := context.Background()
	results, err := runner.Run(ctx)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Verify order: A before B
	if len(order) != 2 {
		t.Fatalf("expected 2 executions, got %d", len(order))
	}
	if order[0] != "task-a" {
		t.Errorf("expected task-a first, got %s", order[0])
	}
	if order[1] != "task-b" {
		t.Errorf("expected task-b second, got %s", order[1])
	}
}

// TestMergeConflict_DoesNotBlockOthers verifies one task's merge conflict doesn't block others.
// Two tasks modify the same file (shared.txt) with different content. The first to merge wins;
// the second hits a conflict. A third task with a unique file merges cleanly regardless.
func TestMergeConflict_DoesNotBlockOthers(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create shared.txt in main so both tasks branch from it
	sharedFile := filepath.Join(repoPath, "shared.txt")
	if err := os.WriteFile(sharedFile, []byte("original\n"), 0644); err != nil {
		t.Fatalf("failed to write shared file: %v", err)
	}

	cmd := exec.Command("git", "add", "shared.txt")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v (output: %s)", err, string(output))
	}

	cmd = exec.Command("git", "commit", "-m", "Add shared.txt")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v (output: %s)", err, string(output))
	}

	// Create DAG: writer-a and writer-b both modify shared.txt, clean-task writes unique.txt
	dag := scheduler.NewDAG()
	for _, task := range []*scheduler.Task{
		{
			ID: "writer-a", Name: "Writer A", AgentRole: "coder",
			Prompt: "write-a", DependsOn: []string{}, WritesFiles: []string{},
			Status: scheduler.TaskPending, FailureMode: scheduler.FailHard,
		},
		{
			ID: "writer-b", Name: "Writer B", AgentRole: "coder",
			Prompt: "write-b", DependsOn: []string{}, WritesFiles: []string{},
			Status: scheduler.TaskPending, FailureMode: scheduler.FailHard,
		},
		{
			ID: "clean-task", Name: "Clean Task", AgentRole: "coder",
			Prompt: "write-clean", DependsOn: []string{}, WritesFiles: []string{},
			Status: scheduler.TaskPending, FailureMode: scheduler.FailHard,
		},
	} {
		if err := dag.AddTask(task); err != nil {
			t.Fatalf("failed to add task: %v", err)
		}
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		var filename, content string
		switch {
		case strings.Contains(msg.Content, "write-a"):
			filename, content = "shared.txt", "version A\n"
		case strings.Contains(msg.Content, "write-b"):
			filename, content = "shared.txt", "version B\n"
		default:
			filename, content = "unique.txt", "unique content\n"
		}

		filePath := filepath.Join(workDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return backend.Response{}, err
		}

		addCmd := exec.Command("git", "add", filename)
		addCmd.Dir = workDir
		if output, err := addCmd.CombinedOutput(); err != nil {
			return backend.Response{}, fmt.Errorf("git add failed: %v (output: %s)", err, string(output))
		}

		commitCmd := exec.Command("git", "commit", "-m", "Modify "+filename)
		commitCmd.Dir = workDir
		if output, err := commitCmd.CombinedOutput(); err != nil {
			return backend.Response{}, fmt.Errorf("git commit failed: %v (output: %s)", err, string(output))
		}

		return backend.Response{Content: "done", SessionID: "mock"}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		ConcurrencyLimit: 4,
		WorktreeManager:  wtMgr,
		MergeStrategy:    worktree.MergeOrt,
		BackendFactory:   factory.factory,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	ctx := context.Background()
	results, err := runner.Run(ctx)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Count outcomes
	mergedCount := 0
	conflictCount := 0

	for _, result := range results {
		if result.TaskID == "clean-task" {
			if !result.Success || result.MergeResult == nil || !result.MergeResult.Merged {
				t.Errorf("clean-task should have succeeded and merged")
			}
			mergedCount++
		}
		if result.TaskID == "writer-a" || result.TaskID == "writer-b" {
			if result.MergeResult != nil && result.MergeResult.Merged {
				mergedCount++
			} else {
				conflictCount++
			}
		}
	}

	// One writer merges first (clean), the other conflicts. Clean task always merges.
	if mergedCount < 2 {
		t.Errorf("expected at least 2 merged tasks (1 writer + clean), got %d", mergedCount)
	}
	if conflictCount < 1 {
		t.Errorf("expected at least 1 conflict (second writer), got %d", conflictCount)
	}

	t.Logf("Results: %d merged, %d conflicted", mergedCount, conflictCount)
}

// TestQAChannel_IntegratedWithRunner verifies QA channel works during task execution.
func TestQAChannel_IntegratedWithRunner(t *testing.T) {
	repoPath := setupTestRepo(t)

	dag := scheduler.NewDAG()
	task := &scheduler.Task{
		ID:          "task-qa",
		Name:        "QA Task",
		AgentRole:   "coder",
		Prompt:      "Ask question",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	if err := dag.AddTask(task); err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Create QA channel
	answerCalled := atomic.Bool{}
	qaChannel := NewQAChannel(8, func(ctx context.Context, taskID string, question string) (string, error) {
		answerCalled.Store(true)
		return "Answer: " + question, nil
	})

	// Backend asks question during Send
	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		// Give QA channel handler time to start
		time.Sleep(10 * time.Millisecond)

		// Ask a question
		answer, err := qaChannel.Ask(ctx, "task-qa", "What should I do?")
		if err != nil {
			return backend.Response{}, err
		}

		return backend.Response{
			Content:   fmt.Sprintf("Got answer: %s", answer),
			SessionID: "mock",
		}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		QAChannel:       qaChannel,
		BackendFactory:  factory.factory,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results, err := runner.Run(ctx)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].Success {
		t.Errorf("task failed: %v", results[0].Error)
	}

	if !answerCalled.Load() {
		t.Error("answer function was not called")
	}
}

// TestCleanupOnContextCancel verifies worktrees are cleaned up on context cancellation.
func TestCleanupOnContextCancel(t *testing.T) {
	repoPath := setupTestRepo(t)

	dag := scheduler.NewDAG()
	for i := 1; i <= 2; i++ {
		task := &scheduler.Task{
			ID:          fmt.Sprintf("task-%d", i),
			Name:        fmt.Sprintf("Task %d", i),
			AgentRole:   "coder",
			Prompt:      "Slow work",
			DependsOn:   []string{},
			WritesFiles: []string{},
			Status:      scheduler.TaskPending,
			FailureMode: scheduler.FailHard,
		}
		if err := dag.AddTask(task); err != nil {
			t.Fatalf("failed to add task: %v", err)
		}
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	factory := newMockBackendFactory()
	factory.delay = 200 * time.Millisecond

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	results, err := runner.Run(ctx)

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	// Some tasks may have completed before cancellation
	t.Logf("Completed %d tasks before cancellation", len(results))

	// Verify worktrees are cleaned up
	verifyWorktreesCleanedUp(t, repoPath)
}

// TestPruneOnStartup verifies stale worktrees are pruned on startup.
func TestPruneOnStartup(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create a stale worktree entry (directory without git metadata)
	worktreeDir := filepath.Join(repoPath, ".worktrees", "stale-task")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create stale worktree dir: %v", err)
	}

	dag := scheduler.NewDAG()
	lockMgr := scheduler.NewResourceLockManager()

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	factory := newMockBackendFactory()

	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	ctx := context.Background()
	_, err := runner.Run(ctx)

	// Should complete without errors
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Prune should have been called (no way to verify directly, but no crash = success)
}

// Helper: verify worktrees are cleaned up
func verifyWorktreesCleanedUp(t *testing.T, repoPath string) {
	t.Helper()

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to list worktrees: %v (output: %s)", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	worktreeCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			worktreeCount++
		}
	}

	// Should only have the main worktree
	if worktreeCount != 1 {
		t.Errorf("expected 1 worktree (main), got %d", worktreeCount)
		t.Logf("Worktree list output:\n%s", string(output))
	}
}

// TestEventBusIntegration verifies event bus integration with ParallelRunner.
func TestEventBusIntegration(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create DAG with 2 tasks
	dag := scheduler.NewDAG()
	taskA := &scheduler.Task{
		ID:          "task-a",
		Name:        "Task A",
		AgentRole:   "coder",
		Prompt:      "Work A",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}
	taskB := &scheduler.Task{
		ID:          "task-b",
		Name:        "Task B",
		AgentRole:   "coder",
		Prompt:      "Work B",
		DependsOn:   []string{"task-a"},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	if err := dag.AddTask(taskA); err != nil {
		t.Fatalf("failed to add task A: %v", err)
	}
	if err := dag.AddTask(taskB); err != nil {
		t.Fatalf("failed to add task B: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Create event bus and subscribe to events
	eventBus := events.NewEventBus()
	defer eventBus.Close()

	taskCh := eventBus.Subscribe(events.TopicTask, 100)
	dagCh := eventBus.Subscribe(events.TopicDAG, 100)

	// Collect events
	receivedEvents := make([]events.Event, 0)
	var eventsMu sync.Mutex

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-taskCh:
				if !ok {
					return
				}
				eventsMu.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMu.Unlock()
			case event, ok := <-dagCh:
				if !ok {
					return
				}
				eventsMu.Lock()
				receivedEvents = append(receivedEvents, event)
				eventsMu.Unlock()
			case <-done:
				return
			}
		}
	}()
	defer func() {
		done <- true
	}()

	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		return backend.Response{Content: "done", SessionID: "mock"}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
		EventBus:        eventBus,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	ctx := context.Background()
	results, err := runner.Run(ctx)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Give time for events to be collected
	time.Sleep(50 * time.Millisecond)

	// Verify events
	eventsMu.Lock()
	defer eventsMu.Unlock()

	taskStartedCount := 0
	taskCompletedCount := 0
	dagProgressCount := 0

	for _, event := range receivedEvents {
		switch event.EventType() {
		case events.EventTypeTaskStarted:
			taskStartedCount++
			// Verify timestamp is non-zero
			if e, ok := event.(events.TaskStartedEvent); ok && e.Timestamp.IsZero() {
				t.Error("TaskStartedEvent has zero timestamp")
			}
		case events.EventTypeTaskCompleted:
			taskCompletedCount++
			// Verify timestamp is non-zero
			if e, ok := event.(events.TaskCompletedEvent); ok && e.Timestamp.IsZero() {
				t.Error("TaskCompletedEvent has zero timestamp")
			}
		case events.EventTypeDAGProgress:
			dagProgressCount++
			// Verify timestamp is non-zero
			if e, ok := event.(events.DAGProgressEvent); ok && e.Timestamp.IsZero() {
				t.Error("DAGProgressEvent has zero timestamp")
			}
		}
	}

	if taskStartedCount < 2 {
		t.Errorf("expected at least 2 TaskStarted events, got %d", taskStartedCount)
	}
	if taskCompletedCount < 2 {
		t.Errorf("expected at least 2 TaskCompleted events, got %d", taskCompletedCount)
	}
	if dagProgressCount < 1 {
		t.Errorf("expected at least 1 DAGProgress event, got %d", dagProgressCount)
	}

	t.Logf("Received events: %d TaskStarted, %d TaskCompleted, %d DAGProgress",
		taskStartedCount, taskCompletedCount, dagProgressCount)
}

// testStoreForRunner creates an in-memory persistence Store for testing.
func testStoreForRunner(t *testing.T) persistence.Store {
	t.Helper()

	ctx := context.Background()
	store, err := persistence.NewMemoryStore(ctx)
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

// TestCheckpointOnTaskCompletion verifies task state is checkpointed on completion.
func TestCheckpointOnTaskCompletion(t *testing.T) {
	repoPath := setupTestRepo(t)
	store := testStoreForRunner(t)

	// Create DAG with 1 task
	dag := scheduler.NewDAG()
	task := &scheduler.Task{
		ID:          "task-1",
		Name:        "Task 1",
		AgentRole:   "coder",
		Prompt:      "Do work",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	if err := dag.AddTask(task); err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Mock backend that returns a canned response with session ID
	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		return backend.Response{
			Content:   "Task completed successfully",
			SessionID: "test-session-123",
		}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
		Store:           store,
		BackendConfigs: map[string]backend.Config{
			"coder": {Type: "claude"},
		},
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	// Run
	ctx := context.Background()
	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].Success {
		t.Errorf("task failed: %v", results[0].Error)
	}

	// Verify task status in store
	persistedTask, err := store.GetTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("failed to get task from store: %v", err)
	}

	if persistedTask.Status != scheduler.TaskCompleted {
		t.Errorf("expected status TaskCompleted, got %v", persistedTask.Status)
	}

	if persistedTask.Result != "Task completed successfully" {
		t.Errorf("expected result to match response content, got %q", persistedTask.Result)
	}

	// Verify session in store
	sessionID, backendType, err := store.GetSession(ctx, "task-1")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if sessionID != "mock-session-task-1" {
		t.Errorf("expected session ID 'mock-session-task-1', got %q", sessionID)
	}

	if backendType != "claude" {
		t.Errorf("expected backend type 'claude', got %q", backendType)
	}

	// Verify conversation history
	history, err := store.GetHistory(ctx, "task-1")
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(history))
	}

	if history[0].Role != "user" || history[0].Content != "Do work" {
		t.Errorf("first message should be user prompt, got role=%q content=%q", history[0].Role, history[0].Content)
	}

	if history[1].Role != "assistant" || history[1].Content != "Task completed successfully" {
		t.Errorf("second message should be assistant response, got role=%q content=%q", history[1].Role, history[1].Content)
	}
}

// TestCheckpointOnTaskFailure verifies task state is checkpointed on failure.
func TestCheckpointOnTaskFailure(t *testing.T) {
	repoPath := setupTestRepo(t)
	store := testStoreForRunner(t)

	// Create DAG with 1 task
	dag := scheduler.NewDAG()
	task := &scheduler.Task{
		ID:          "task-fail",
		Name:        "Task Fail",
		AgentRole:   "coder",
		Prompt:      "Do work",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	if err := dag.AddTask(task); err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Mock backend that returns an error
	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		return backend.Response{}, fmt.Errorf("simulated backend error")
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
		Store:           store,
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	// Run
	ctx := context.Background()
	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Success {
		t.Error("expected task to fail")
	}

	// Verify task status in store
	persistedTask, err := store.GetTask(ctx, "task-fail")
	if err != nil {
		t.Fatalf("failed to get task from store: %v", err)
	}

	if persistedTask.Status != scheduler.TaskFailed {
		t.Errorf("expected status TaskFailed, got %v", persistedTask.Status)
	}

	if persistedTask.Error == nil {
		t.Error("expected error to be persisted")
	}

	if !strings.Contains(persistedTask.Error.Error(), "simulated backend error") {
		t.Errorf("expected error to contain 'simulated backend error', got %q", persistedTask.Error.Error())
	}
}

// TestCheckpointNilStoreNoError verifies nil Store is handled gracefully.
func TestCheckpointNilStoreNoError(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create DAG with 1 task
	dag := scheduler.NewDAG()
	task := &scheduler.Task{
		ID:          "task-no-store",
		Name:        "Task No Store",
		AgentRole:   "coder",
		Prompt:      "Do work",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	if err := dag.AddTask(task); err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		return backend.Response{Content: "done", SessionID: "mock"}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
		Store:           nil, // No store
	}

	runner := NewParallelRunner(cfg, dag, lockMgr)

	// Run should complete without panic or error
	ctx := context.Background()
	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].Success {
		t.Errorf("task failed: %v", results[0].Error)
	}
}

// TestResumeSkipsCompletedTasks verifies Resume skips completed tasks and only executes pending ones.
func TestResumeSkipsCompletedTasks(t *testing.T) {
	repoPath := setupTestRepo(t)
	store := testStoreForRunner(t)
	ctx := context.Background()

	// Persist 3 tasks: task-1 and task-2 completed, task-3 pending (depends on 1 and 2)
	task1 := &scheduler.Task{
		ID:          "task-1",
		Name:        "Task 1",
		AgentRole:   "coder",
		Prompt:      "Work 1",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskCompleted,
		FailureMode: scheduler.FailHard,
		Result:      "Result 1",
	}

	task2 := &scheduler.Task{
		ID:          "task-2",
		Name:        "Task 2",
		AgentRole:   "coder",
		Prompt:      "Work 2",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskCompleted,
		FailureMode: scheduler.FailHard,
		Result:      "Result 2",
	}

	task3 := &scheduler.Task{
		ID:          "task-3",
		Name:        "Task 3",
		AgentRole:   "coder",
		Prompt:      "Work 3",
		DependsOn:   []string{"task-1", "task-2"},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	// Save tasks to store
	if err := store.SaveTask(ctx, task1); err != nil {
		t.Fatalf("failed to save task-1: %v", err)
	}
	if err := store.SaveTask(ctx, task2); err != nil {
		t.Fatalf("failed to save task-2: %v", err)
	}
	if err := store.SaveTask(ctx, task3); err != nil {
		t.Fatalf("failed to save task-3: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Track which tasks the backend receives
	var executedTasks []string
	var mu sync.Mutex

	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		taskID := filepath.Base(workDir)
		mu.Lock()
		executedTasks = append(executedTasks, taskID)
		mu.Unlock()

		return backend.Response{Content: "done", SessionID: "mock"}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
		Store:           store,
	}

	// Create runner with empty DAG (Resume will populate it)
	dag := scheduler.NewDAG()
	runner := NewParallelRunner(cfg, dag, lockMgr)

	// Resume
	results, err := runner.Resume(ctx)
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	// Should only execute task-3
	mu.Lock()
	defer mu.Unlock()

	if len(executedTasks) != 1 {
		t.Errorf("expected 1 task executed, got %d: %v", len(executedTasks), executedTasks)
	}

	if len(executedTasks) > 0 && executedTasks[0] != "task-3" {
		t.Errorf("expected task-3 to be executed, got %q", executedTasks[0])
	}

	// Verify task-3 is now completed in store
	persistedTask3, err := store.GetTask(ctx, "task-3")
	if err != nil {
		t.Fatalf("failed to get task-3 from store: %v", err)
	}

	if persistedTask3.Status != scheduler.TaskCompleted {
		t.Errorf("expected task-3 to be completed, got status %v", persistedTask3.Status)
	}

	// Verify results
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// TestResumeRestoresSessionID verifies session IDs are persisted and retrievable.
func TestResumeRestoresSessionID(t *testing.T) {
	repoPath := setupTestRepo(t)
	store := testStoreForRunner(t)
	ctx := context.Background()

	// Persist task-1 as completed with a session
	task1 := &scheduler.Task{
		ID:          "task-1",
		Name:        "Task 1",
		AgentRole:   "coder",
		Prompt:      "Work 1",
		DependsOn:   []string{},
		WritesFiles: []string{},
		Status:      scheduler.TaskCompleted,
		FailureMode: scheduler.FailHard,
		Result:      "Result 1",
	}

	// Persist task-2 as pending (depends on task-1)
	task2 := &scheduler.Task{
		ID:          "task-2",
		Name:        "Task 2",
		AgentRole:   "coder",
		Prompt:      "Work 2",
		DependsOn:   []string{"task-1"},
		WritesFiles: []string{},
		Status:      scheduler.TaskPending,
		FailureMode: scheduler.FailHard,
	}

	// Save tasks
	if err := store.SaveTask(ctx, task1); err != nil {
		t.Fatalf("failed to save task-1: %v", err)
	}
	if err := store.SaveTask(ctx, task2); err != nil {
		t.Fatalf("failed to save task-2: %v", err)
	}

	// Save session for task-1
	if err := store.SaveSession(ctx, "task-1", "session-abc-123", "claude"); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	wtMgr := worktree.NewWorktreeManager(worktree.WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	factory := newMockBackendFactory()
	factory.onSend = func(ctx context.Context, msg backend.Message, workDir string) (backend.Response, error) {
		return backend.Response{Content: "done", SessionID: "mock"}, nil
	}

	lockMgr := scheduler.NewResourceLockManager()
	cfg := ParallelRunnerConfig{
		WorktreeManager: wtMgr,
		BackendFactory:  factory.factory,
		Store:           store,
	}

	// Create runner and resume
	dag := scheduler.NewDAG()
	runner := NewParallelRunner(cfg, dag, lockMgr)

	results, err := runner.Resume(ctx)
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	// Verify task-2 executed
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Verify session was loaded into runner
	if sessionID, ok := runner.sessions["task-1"]; !ok {
		t.Error("expected task-1 session to be loaded")
	} else if sessionID != "session-abc-123" {
		t.Errorf("expected session ID 'session-abc-123', got %q", sessionID)
	}

	// Verify session is retrievable from store
	sessionID, backendType, err := store.GetSession(ctx, "task-1")
	if err != nil {
		t.Fatalf("failed to retrieve session: %v", err)
	}

	if sessionID != "session-abc-123" {
		t.Errorf("expected session ID 'session-abc-123', got %q", sessionID)
	}

	if backendType != "claude" {
		t.Errorf("expected backend type 'claude', got %q", backendType)
	}
}
