package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aristath/orchestrator/internal/backend"
	"github.com/aristath/orchestrator/internal/config"
)

// TestIntegration_FullPipeline validates the end-to-end flow:
// config -> DAG -> execute -> workflow follow-up -> execute follow-up
func TestIntegration_FullPipeline(t *testing.T) {
	// 1. Load default config
	cfg := config.DefaultConfig()

	// Verify config has expected structure
	if cfg.Agents["coder"].Provider != "claude" {
		t.Errorf("expected coder to use claude provider, got %q", cfg.Agents["coder"].Provider)
	}
	if len(cfg.Workflows["standard"].Steps) != 3 {
		t.Errorf("expected 3 steps in standard workflow, got %d", len(cfg.Workflows["standard"].Steps))
	}

	// 2. Create DAG with 3 tasks
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:          "implement-auth",
		Name:        "Implement auth",
		AgentRole:   "coder",
		Prompt:      "implement authentication",
		WritesFiles: []string{"auth.go"},
		Status:      TaskPending,
		FailureMode: FailHard,
	})
	_ = dag.AddTask(&Task{
		ID:          "implement-db",
		Name:        "Implement database",
		AgentRole:   "coder",
		Prompt:      "implement database layer",
		WritesFiles: []string{"db.go"},
		Status:      TaskPending,
		FailureMode: FailHard,
	})
	_ = dag.AddTask(&Task{
		ID:          "implement-api",
		Name:        "Implement API",
		AgentRole:   "coder",
		Prompt:      "implement API endpoints",
		DependsOn:   []string{"implement-auth", "implement-db"},
		WritesFiles: []string{"api.go"},
		Status:      TaskPending,
		FailureMode: FailHard,
	})

	// 3. Validate DAG
	order, err := dag.Validate()
	if err != nil {
		t.Fatalf("DAG validation failed: %v", err)
	}

	// Verify order: auth and db before api
	apiIndex := -1
	authIndex := -1
	dbIndex := -1
	for i, taskID := range order {
		switch taskID {
		case "implement-api":
			apiIndex = i
		case "implement-auth":
			authIndex = i
		case "implement-db":
			dbIndex = i
		}
	}
	if authIndex > apiIndex || dbIndex > apiIndex {
		t.Errorf("expected auth (%d) and db (%d) before api (%d)", authIndex, dbIndex, apiIndex)
	}

	// 4. Create Executor with mock backends
	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)

	// Register mock backends for all roles
	exec.RegisterBackend("coder", &mockBackend{
		response: backend.Response{Content: "code implemented"},
	})
	exec.RegisterBackend("reviewer", &mockBackend{
		response: backend.Response{Content: "code reviewed"},
	})
	exec.RegisterBackend("tester", &mockBackend{
		response: backend.Response{Content: "tests written"},
	})

	// 5. Create WorkflowManager with standard workflow
	workflowMgr := NewWorkflowManager(dag, cfg.Workflows)

	// 6. Get eligible tasks - should be auth and db
	eligible := exec.NextEligible()
	if len(eligible) != 2 {
		t.Fatalf("expected 2 eligible tasks, got %d", len(eligible))
	}

	eligibleIDs := make(map[string]bool)
	for _, task := range eligible {
		eligibleIDs[task.ID] = true
	}
	if !eligibleIDs["implement-auth"] || !eligibleIDs["implement-db"] {
		t.Errorf("expected auth and db to be eligible, got: %v", eligibleIDs)
	}

	// 7. Execute both eligible tasks
	for _, task := range eligible {
		if err := exec.ExecuteTask(context.Background(), task.ID); err != nil {
			t.Fatalf("failed to execute %s: %v", task.ID, err)
		}

		// 8. After each completion, call WorkflowManager
		completedTask, _ := dag.Get(task.ID)
		newTasks, err := workflowMgr.OnTaskCompleted(completedTask)
		if err != nil {
			t.Fatalf("workflow manager failed for %s: %v", task.ID, err)
		}

		// Should spawn reviewer follow-ups
		if len(newTasks) != 1 {
			t.Errorf("expected 1 follow-up for %s, got %d", task.ID, len(newTasks))
		}
		if len(newTasks) > 0 && newTasks[0].AgentRole != "reviewer" {
			t.Errorf("expected reviewer follow-up, got %s", newTasks[0].AgentRole)
		}
	}

	// 9. Get eligible again - should include implement-api and reviewer follow-ups
	eligible = exec.NextEligible()
	eligibleCount := len(eligible)

	// Should have: implement-api + 2 reviewer follow-ups = 3
	if eligibleCount != 3 {
		t.Errorf("expected 3 eligible tasks (1 api + 2 reviewers), got %d", eligibleCount)
	}

	// Find and execute implement-api
	var apiTask *Task
	for _, task := range eligible {
		if task.ID == "implement-api" {
			apiTask = task
			break
		}
	}
	if apiTask == nil {
		t.Fatal("implement-api should be eligible")
	}

	// 10. Execute implement-api
	if err := exec.ExecuteTask(context.Background(), apiTask.ID); err != nil {
		t.Fatalf("failed to execute api task: %v", err)
	}

	// Spawn follow-up for api task
	completedAPI, _ := dag.Get(apiTask.ID)
	apiFollowUps, err := workflowMgr.OnTaskCompleted(completedAPI)
	if err != nil {
		t.Fatalf("workflow manager failed for api: %v", err)
	}
	if len(apiFollowUps) != 1 {
		t.Errorf("expected 1 follow-up for api, got %d", len(apiFollowUps))
	}

	// 11. Verify reviewer follow-ups exist in DAG
	reviewerFollowUps := 0
	for _, task := range dag.Tasks() {
		if task.AgentRole == "reviewer" {
			reviewerFollowUps++
		}
	}
	if reviewerFollowUps != 3 {
		t.Errorf("expected 3 reviewer follow-ups, got %d", reviewerFollowUps)
	}

	// 12. Execute a reviewer follow-up, verify tester follow-up is created
	eligible = exec.NextEligible()
	var reviewerTask *Task
	for _, task := range eligible {
		if task.AgentRole == "reviewer" {
			reviewerTask = task
			break
		}
	}
	if reviewerTask == nil {
		t.Fatal("reviewer task should be eligible")
	}

	if err := exec.ExecuteTask(context.Background(), reviewerTask.ID); err != nil {
		t.Fatalf("failed to execute reviewer task: %v", err)
	}

	completedReviewer, _ := dag.Get(reviewerTask.ID)
	testerTasks, err := workflowMgr.OnTaskCompleted(completedReviewer)
	if err != nil {
		t.Fatalf("workflow manager failed for reviewer: %v", err)
	}

	// Should spawn tester follow-up
	if len(testerTasks) != 1 {
		t.Errorf("expected 1 tester follow-up, got %d", len(testerTasks))
	}
	if len(testerTasks) > 0 && testerTasks[0].AgentRole != "tester" {
		t.Errorf("expected tester follow-up, got %s", testerTasks[0].AgentRole)
	}

	// 13. Verify final state: all original tasks completed, follow-up chain exists
	authTask, _ := dag.Get("implement-auth")
	dbTask, _ := dag.Get("implement-db")
	apiTaskFinal, _ := dag.Get("implement-api")

	if authTask.Status != TaskCompleted {
		t.Errorf("expected auth completed, got %d", authTask.Status)
	}
	if dbTask.Status != TaskCompleted {
		t.Errorf("expected db completed, got %d", dbTask.Status)
	}
	if apiTaskFinal.Status != TaskCompleted {
		t.Errorf("expected api completed, got %d", apiTaskFinal.Status)
	}

	// Verify tester task exists
	testerCount := 0
	for _, task := range dag.Tasks() {
		if task.AgentRole == "tester" {
			testerCount++
		}
	}
	if testerCount < 1 {
		t.Errorf("expected at least 1 tester follow-up, got %d", testerCount)
	}
}

// TestIntegration_ResourceLockContention validates that file locks prevent concurrent writes.
func TestIntegration_ResourceLockContention(t *testing.T) {
	// 1. Create DAG with 2 tasks writing the SAME file
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:          "task-a",
		Name:        "Write shared file A",
		AgentRole:   "coder",
		Prompt:      "write shared.go first",
		WritesFiles: []string{"shared.go"},
		Status:      TaskPending,
		FailureMode: FailHard,
	})
	_ = dag.AddTask(&Task{
		ID:          "task-b",
		Name:        "Write shared file B",
		AgentRole:   "coder",
		Prompt:      "write shared.go second",
		WritesFiles: []string{"shared.go"},
		Status:      TaskPending,
		FailureMode: FailHard,
	})

	_, err := dag.Validate()
	if err != nil {
		t.Fatalf("DAG validation failed: %v", err)
	}

	// 2. Both are eligible (no dependencies)
	eligible := dag.Eligible()
	if len(eligible) != 2 {
		t.Fatalf("expected 2 eligible tasks, got %d", len(eligible))
	}

	// 3. Create executor with tracking backend
	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)

	var maxConcurrent atomic.Int32
	var current atomic.Int32

	// Mock backend with 50ms delay
	trackingBackend := &concurrentTrackingBackend{
		sendDelay:     50 * time.Millisecond,
		response:      backend.Response{Content: "done"},
		current:       &current,
		maxConcurrent: &maxConcurrent,
	}
	exec.RegisterBackend("coder", trackingBackend)

	// 4. Launch both ExecuteTask calls in separate goroutines
	done := make(chan struct{}, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = exec.ExecuteTask(context.Background(), "task-a")
		done <- struct{}{}
	}()
	go func() {
		defer wg.Done()
		_ = exec.ExecuteTask(context.Background(), "task-b")
		done <- struct{}{}
	}()

	wg.Wait()

	// 5. Verify both completed
	taskA, _ := dag.Get("task-a")
	taskB, _ := dag.Get("task-b")

	if taskA.Status != TaskCompleted {
		t.Errorf("expected task-a completed, got %d", taskA.Status)
	}
	if taskB.Status != TaskCompleted {
		t.Errorf("expected task-b completed, got %d", taskB.Status)
	}

	// 6. Verify they did NOT run simultaneously (resource lock prevented it)
	if maxConcurrent.Load() > 1 {
		t.Errorf("expected max concurrent 1 (file lock should serialize), got %d", maxConcurrent.Load())
	}
}

// concurrentTrackingBackend tracks concurrent Send() calls.
type concurrentTrackingBackend struct {
	sendDelay     time.Duration
	response      backend.Response
	current       *atomic.Int32
	maxConcurrent *atomic.Int32
}

func (c *concurrentTrackingBackend) Send(ctx context.Context, msg backend.Message) (backend.Response, error) {
	cur := c.current.Add(1)

	// Update max concurrent
	for {
		max := c.maxConcurrent.Load()
		if cur <= max {
			break
		}
		if c.maxConcurrent.CompareAndSwap(max, cur) {
			break
		}
	}

	// Simulate work
	select {
	case <-time.After(c.sendDelay):
	case <-ctx.Done():
		c.current.Add(-1)
		return backend.Response{}, ctx.Err()
	}

	c.current.Add(-1)
	return c.response, nil
}

func (c *concurrentTrackingBackend) Close() error     { return nil }
func (c *concurrentTrackingBackend) SessionID() string { return "concurrent-tracking" }

// TestIntegration_FailurePropagation validates FailHard blocks, FailSoft allows.
func TestIntegration_FailurePropagation(t *testing.T) {
	// 1. Create DAG: "build" (FailHard) -> "deploy", "lint" (FailSoft) -> "test"
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:          "build",
		Name:        "Build",
		AgentRole:   "coder",
		Prompt:      "build the project",
		Status:      TaskPending,
		FailureMode: FailHard,
	})
	_ = dag.AddTask(&Task{
		ID:          "deploy",
		Name:        "Deploy",
		AgentRole:   "coder",
		Prompt:      "deploy the project",
		DependsOn:   []string{"build"},
		Status:      TaskPending,
		FailureMode: FailHard,
	})
	_ = dag.AddTask(&Task{
		ID:          "lint",
		Name:        "Lint",
		AgentRole:   "reviewer",
		Prompt:      "lint the code",
		Status:      TaskPending,
		FailureMode: FailSoft,
	})
	_ = dag.AddTask(&Task{
		ID:          "test",
		Name:        "Test",
		AgentRole:   "tester",
		Prompt:      "run tests",
		DependsOn:   []string{"lint"},
		Status:      TaskPending,
		FailureMode: FailHard,
	})

	_, err := dag.Validate()
	if err != nil {
		t.Fatalf("DAG validation failed: %v", err)
	}

	// 2. Create executor with failing and succeeding backends
	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)

	// Build will fail
	exec.RegisterBackend("coder", &mockBackend{
		err: fmt.Errorf("build failed"),
	})
	// Lint will fail but is FailSoft
	exec.RegisterBackend("reviewer", &mockBackend{
		err: fmt.Errorf("lint failed"),
	})
	exec.RegisterBackend("tester", &mockBackend{
		response: backend.Response{Content: "tests passed"},
	})

	// 3. Execute "build" with failure
	if err := exec.ExecuteTask(context.Background(), "build"); err != nil {
		t.Fatalf("ExecuteTask should return nil (failure stored in DAG), got: %v", err)
	}

	buildTask, _ := dag.Get("build")
	if buildTask.Status != TaskFailed {
		t.Errorf("expected build to fail, got %d", buildTask.Status)
	}

	// 4. Verify "deploy" is NOT eligible (hard failure blocks)
	eligible := exec.NextEligible()
	for _, task := range eligible {
		if task.ID == "deploy" {
			t.Error("deploy should NOT be eligible after build FailHard")
		}
	}

	// 5. Execute "lint" with failure
	if err := exec.ExecuteTask(context.Background(), "lint"); err != nil {
		t.Fatalf("ExecuteTask should return nil, got: %v", err)
	}

	lintTask, _ := dag.Get("lint")
	if lintTask.Status != TaskFailed {
		t.Errorf("expected lint to fail, got %d", lintTask.Status)
	}

	// 6. Verify "test" IS eligible (soft failure allows)
	eligible = exec.NextEligible()
	testEligible := false
	for _, task := range eligible {
		if task.ID == "test" {
			testEligible = true
		}
	}
	if !testEligible {
		t.Error("test SHOULD be eligible after lint FailSoft")
	}
}

// TestIntegration_ConfigDrivenBackendSelection validates config-to-backend routing.
func TestIntegration_ConfigDrivenBackendSelection(t *testing.T) {
	// 1. Create custom config with multiple providers
	cfg := &config.OrchestratorConfig{
		Providers: map[string]config.ProviderConfig{
			"claude": {
				Type:    "claude",
				Command: "claude",
			},
			"codex": {
				Type:    "codex",
				Command: "codex",
			},
		},
		Agents: map[string]config.AgentConfig{
			"coder": {
				Provider:     "claude",
				SystemPrompt: "You write code",
			},
			"reviewer": {
				Provider:     "codex",
				SystemPrompt: "You review code",
			},
		},
		Workflows: map[string]config.WorkflowConfig{},
	}

	// 2. Verify config structure
	if cfg.Agents["coder"].Provider != "claude" {
		t.Errorf("expected coder to use claude, got %q", cfg.Agents["coder"].Provider)
	}
	if cfg.Agents["reviewer"].Provider != "codex" {
		t.Errorf("expected reviewer to use codex, got %q", cfg.Agents["reviewer"].Provider)
	}

	claudeProviderType := cfg.Providers[cfg.Agents["coder"].Provider].Type
	codexProviderType := cfg.Providers[cfg.Agents["reviewer"].Provider].Type
	if claudeProviderType != "claude" {
		t.Errorf("expected claude provider type, got %q", claudeProviderType)
	}
	if codexProviderType != "codex" {
		t.Errorf("expected codex provider type, got %q", codexProviderType)
	}

	// 3. Create DAG with tasks using different agent roles
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:          "code-task",
		Name:        "Code something",
		AgentRole:   "coder",
		Prompt:      "write code",
		Status:      TaskPending,
		FailureMode: FailHard,
	})
	_ = dag.AddTask(&Task{
		ID:          "review-task",
		Name:        "Review something",
		AgentRole:   "reviewer",
		Prompt:      "review code",
		Status:      TaskPending,
		FailureMode: FailHard,
	})

	_, err := dag.Validate()
	if err != nil {
		t.Fatalf("DAG validation failed: %v", err)
	}

	// 4. Create mock backends that record which role invoked them
	claudeBackend := &recordingBackend{
		response: backend.Response{Content: "claude response"},
		role:     "claude",
	}
	codexBackend := &recordingBackend{
		response: backend.Response{Content: "codex response"},
		role:     "codex",
	}

	// 5. Create executor and register backends by agent role
	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)
	exec.RegisterBackend("coder", claudeBackend)
	exec.RegisterBackend("reviewer", codexBackend)

	// 6. Execute both tasks
	if err := exec.ExecuteTask(context.Background(), "code-task"); err != nil {
		t.Fatalf("failed to execute code-task: %v", err)
	}
	if err := exec.ExecuteTask(context.Background(), "review-task"); err != nil {
		t.Fatalf("failed to execute review-task: %v", err)
	}

	// 7. Verify correct backend was invoked for each role
	if claudeBackend.callCount.Load() != 1 {
		t.Errorf("expected claude backend called once, got %d", claudeBackend.callCount.Load())
	}
	if codexBackend.callCount.Load() != 1 {
		t.Errorf("expected codex backend called once, got %d", codexBackend.callCount.Load())
	}

	// Verify results show correct backend responses
	codeTask, _ := dag.Get("code-task")
	reviewTask, _ := dag.Get("review-task")

	if codeTask.Result != "claude response" {
		t.Errorf("expected 'claude response', got %q", codeTask.Result)
	}
	if reviewTask.Result != "codex response" {
		t.Errorf("expected 'codex response', got %q", reviewTask.Result)
	}
}

// recordingBackend records which tasks invoked it.
type recordingBackend struct {
	response  backend.Response
	role      string
	callCount atomic.Int32
}

func (r *recordingBackend) Send(ctx context.Context, msg backend.Message) (backend.Response, error) {
	r.callCount.Add(1)
	return r.response, nil
}

func (r *recordingBackend) Close() error     { return nil }
func (r *recordingBackend) SessionID() string { return fmt.Sprintf("recording-%s", r.role) }
