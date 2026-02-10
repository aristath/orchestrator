package scheduler

import (
	"context"
	"fmt"

	"github.com/aristath/orchestrator/internal/backend"
)

// Executor runs tasks by dispatching them to backends with resource locking.
type Executor struct {
	dag      *DAG
	lockMgr  *ResourceLockManager
	backends map[string]backend.Backend // agentRole -> backend instance
}

// NewExecutor creates a new Executor.
func NewExecutor(dag *DAG, lockMgr *ResourceLockManager) *Executor {
	return &Executor{
		dag:      dag,
		lockMgr:  lockMgr,
		backends: make(map[string]backend.Backend),
	}
}

// RegisterBackend maps an agent role to a backend instance.
func (e *Executor) RegisterBackend(agentRole string, b backend.Backend) {
	e.backends[agentRole] = b
}

// ExecuteTask runs a single task through its backend with resource locking.
func (e *Executor) ExecuteTask(ctx context.Context, taskID string) error {
	task, exists := e.dag.Get(taskID)
	if !exists {
		return fmt.Errorf("task %q not found", taskID)
	}

	// Verify task is eligible
	if task.Status != TaskPending && task.Status != TaskEligible {
		return fmt.Errorf("task %q is not eligible (status: %d)", taskID, task.Status)
	}

	// Check dependencies are resolved
	for _, depID := range task.DependsOn {
		dep, ok := e.dag.Get(depID)
		if !ok || !e.dag.isDependencyResolved(dep) {
			return fmt.Errorf("task %q has unresolved dependency %q", taskID, depID)
		}
	}

	// Mark as running
	if err := e.dag.MarkRunning(taskID); err != nil {
		return err
	}

	// Look up backend
	b, ok := e.backends[task.AgentRole]
	if !ok {
		err := fmt.Errorf("no backend registered for agent role %q", task.AgentRole)
		_ = e.dag.MarkFailed(taskID, err)
		return err
	}

	// Acquire file locks
	e.lockMgr.LockAll(task.WritesFiles)
	defer e.lockMgr.UnlockAll(task.WritesFiles)

	// Check context before sending
	if err := ctx.Err(); err != nil {
		markErr := fmt.Errorf("context cancelled before execution: %w", err)
		_ = e.dag.MarkFailed(taskID, markErr)
		return markErr
	}

	// Send to backend
	resp, err := b.Send(ctx, backend.Message{Content: task.Prompt, Role: "user"})
	if err != nil {
		_ = e.dag.MarkFailed(taskID, err)
		return nil // Task status is in DAG, not the return value
	}

	// Mark completed
	_ = e.dag.MarkCompleted(taskID, resp.Content)
	return nil
}

// NextEligible returns tasks that are ready to run.
func (e *Executor) NextEligible() []*Task {
	return e.dag.Eligible()
}
