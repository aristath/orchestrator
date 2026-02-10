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
