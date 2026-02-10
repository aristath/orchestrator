package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aristath/orchestrator/internal/backend"
)

// mockBackend implements backend.Backend for testing.
type mockBackend struct {
	response  backend.Response
	err       error
	sendDelay time.Duration
	sendCount atomic.Int32
}

func (m *mockBackend) Send(ctx context.Context, msg backend.Message) (backend.Response, error) {
	m.sendCount.Add(1)
	if m.sendDelay > 0 {
		select {
		case <-time.After(m.sendDelay):
		case <-ctx.Done():
			return backend.Response{}, ctx.Err()
		}
	}
	return m.response, m.err
}

func (m *mockBackend) Close() error     { return nil }
func (m *mockBackend) SessionID() string { return "mock-session" }

func TestExecutor_SuccessfulExecution(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:        "task-1",
		Name:      "Test task",
		AgentRole: "coder",
		Prompt:    "write code",
		Status:    TaskPending,
	})
	_, _ = dag.Validate()

	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)
	exec.RegisterBackend("coder", &mockBackend{
		response: backend.Response{Content: "code written"},
	})

	err := exec.ExecuteTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	task, _ := dag.Get("task-1")
	if task.Status != TaskCompleted {
		t.Errorf("expected TaskCompleted, got %d", task.Status)
	}
	if task.Result != "code written" {
		t.Errorf("expected result 'code written', got %q", task.Result)
	}
}

func TestExecutor_FailedExecution(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:        "task-1",
		Name:      "Test task",
		AgentRole: "coder",
		Prompt:    "write code",
		Status:    TaskPending,
	})
	_, _ = dag.Validate()

	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)
	exec.RegisterBackend("coder", &mockBackend{
		err: fmt.Errorf("backend error"),
	})

	err := exec.ExecuteTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("expected nil return (failure in DAG), got: %v", err)
	}

	task, _ := dag.Get("task-1")
	if task.Status != TaskFailed {
		t.Errorf("expected TaskFailed, got %d", task.Status)
	}
	if task.Error == nil || task.Error.Error() != "backend error" {
		t.Errorf("expected 'backend error', got: %v", task.Error)
	}
}

func TestExecutor_FileLocking(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:          "task-1",
		Name:        "Write main",
		AgentRole:   "coder",
		Prompt:      "write main.go",
		WritesFiles: []string{"main.go", "utils.go"},
		Status:      TaskPending,
	})
	_ = dag.AddTask(&Task{
		ID:          "task-2",
		Name:        "Write main too",
		AgentRole:   "coder",
		Prompt:      "also write main.go",
		WritesFiles: []string{"main.go"},
		Status:      TaskPending,
	})
	_, _ = dag.Validate()

	lockMgr := NewResourceLockManager()
	mb := &mockBackend{
		response:  backend.Response{Content: "done"},
		sendDelay: 50 * time.Millisecond,
	}
	exec := NewExecutor(dag, lockMgr)
	exec.RegisterBackend("coder", mb)

	// Track concurrent execution
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	origSend := mb.response
	concurrentBackend := &trackingBackend{
		inner:         mb,
		current:       &current,
		maxConcurrent: &maxConcurrent,
		response:      origSend,
	}
	exec.RegisterBackend("coder", concurrentBackend)

	// Execute both in parallel
	done := make(chan struct{}, 2)
	go func() {
		_ = exec.ExecuteTask(context.Background(), "task-1")
		done <- struct{}{}
	}()
	go func() {
		_ = exec.ExecuteTask(context.Background(), "task-2")
		done <- struct{}{}
	}()

	<-done
	<-done

	// Both should have completed but NOT concurrently (same file lock)
	if maxConcurrent.Load() > 1 {
		t.Errorf("expected max concurrent 1 (file lock), got %d", maxConcurrent.Load())
	}
}

// trackingBackend wraps a backend and tracks concurrent usage.
type trackingBackend struct {
	inner         *mockBackend
	current       *atomic.Int32
	maxConcurrent *atomic.Int32
	response      backend.Response
}

func (t *trackingBackend) Send(ctx context.Context, msg backend.Message) (backend.Response, error) {
	cur := t.current.Add(1)
	for {
		max := t.maxConcurrent.Load()
		if cur <= max {
			break
		}
		if t.maxConcurrent.CompareAndSwap(max, cur) {
			break
		}
	}
	time.Sleep(50 * time.Millisecond)
	t.current.Add(-1)
	return t.response, nil
}

func (t *trackingBackend) Close() error     { return nil }
func (t *trackingBackend) SessionID() string { return "tracking-session" }

func TestExecutor_UnknownAgentRole(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:        "task-1",
		Name:      "Test task",
		AgentRole: "unknown-role",
		Prompt:    "do something",
		Status:    TaskPending,
	})
	_, _ = dag.Validate()

	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)

	err := exec.ExecuteTask(context.Background(), "task-1")
	if err == nil {
		t.Fatal("expected error for unknown agent role")
	}

	task, _ := dag.Get("task-1")
	if task.Status != TaskFailed {
		t.Errorf("expected TaskFailed, got %d", task.Status)
	}
}

func TestExecutor_NotEligible(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:     "dep-1",
		Name:   "Dependency",
		Status: TaskPending,
	})
	_ = dag.AddTask(&Task{
		ID:        "task-1",
		Name:      "Blocked task",
		AgentRole: "coder",
		Prompt:    "write code",
		DependsOn: []string{"dep-1"},
		Status:    TaskPending,
	})
	_, _ = dag.Validate()

	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)
	exec.RegisterBackend("coder", &mockBackend{
		response: backend.Response{Content: "done"},
	})

	err := exec.ExecuteTask(context.Background(), "task-1")
	if err == nil {
		t.Fatal("expected error for not-eligible task")
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddTask(&Task{
		ID:        "task-1",
		Name:      "Slow task",
		AgentRole: "coder",
		Prompt:    "slow work",
		Status:    TaskPending,
	})
	_, _ = dag.Validate()

	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)
	exec.RegisterBackend("coder", &mockBackend{
		response:  backend.Response{Content: "done"},
		sendDelay: 5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := exec.ExecuteTask(ctx, "task-1")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	task, _ := dag.Get("task-1")
	if task.Status != TaskFailed {
		t.Errorf("expected TaskFailed, got %d", task.Status)
	}
}

func TestExecutor_NextEligible(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddTask(&Task{ID: "a", Name: "A", Status: TaskPending})
	_ = dag.AddTask(&Task{ID: "b", Name: "B", DependsOn: []string{"a"}, Status: TaskPending})
	_, _ = dag.Validate()

	lockMgr := NewResourceLockManager()
	exec := NewExecutor(dag, lockMgr)

	eligible := exec.NextEligible()
	if len(eligible) != 1 || eligible[0].ID != "a" {
		t.Errorf("expected only 'a' eligible, got %v", eligible)
	}
}
