package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/aristath/orchestrator/internal/backend"
	"github.com/aristath/orchestrator/internal/scheduler"
	"github.com/aristath/orchestrator/internal/worktree"
)

// TaskResult represents the outcome of a task execution.
type TaskResult struct {
	TaskID      string
	Success     bool
	MergeResult *worktree.MergeResult
	Error       error
}

// BackendFactory creates backend instances for tasks.
// Parameters: agentRole, workDir (worktree path for the task).
// Returns: Backend instance or error.
type BackendFactory func(agentRole string, workDir string) (backend.Backend, error)

// ParallelRunnerConfig configures the parallel runner.
type ParallelRunnerConfig struct {
	ConcurrencyLimit int                        // Max concurrent tasks (default 4)
	MergeStrategy    worktree.MergeStrategy     // Merge strategy for worktrees
	WorktreeManager  *worktree.WorktreeManager  // Worktree manager instance
	QAChannel        *QAChannel                 // Optional Q&A channel (nil disables)
	ProcessManager   *backend.ProcessManager    // Process manager for backend creation
	BackendConfigs   map[string]backend.Config  // Maps agentRole to base backend config
	BackendFactory   BackendFactory             // Optional factory for testing (overrides BackendConfigs)
}

// ParallelRunner executes DAG tasks concurrently with git worktree isolation.
type ParallelRunner struct {
	config           ParallelRunnerConfig
	dag              *scheduler.DAG
	lockMgr          *scheduler.ResourceLockManager
	mu               sync.Mutex
	activeWorktrees  map[string]*worktree.WorktreeInfo
	results          []TaskResult
}

// NewParallelRunner creates a new parallel runner.
func NewParallelRunner(cfg ParallelRunnerConfig, dag *scheduler.DAG, lockMgr *scheduler.ResourceLockManager) *ParallelRunner {
	if cfg.ConcurrencyLimit <= 0 {
		cfg.ConcurrencyLimit = 4
	}

	return &ParallelRunner{
		config:          cfg,
		dag:             dag,
		lockMgr:         lockMgr,
		activeWorktrees: make(map[string]*worktree.WorktreeInfo),
		results:         []TaskResult{},
	}
}

// Run executes all eligible tasks concurrently with bounded concurrency.
func (r *ParallelRunner) Run(ctx context.Context) ([]TaskResult, error) {
	// Clean stale worktrees from prior crashes
	if err := r.config.WorktreeManager.Prune(); err != nil {
		log.Printf("WARNING: failed to prune stale worktrees: %v", err)
	}

	// Start QA channel if configured
	if r.config.QAChannel != nil {
		r.config.QAChannel.Start(ctx)
		defer r.config.QAChannel.Stop()
	}

	// Cleanup active worktrees on exit (catches shutdown/panic paths)
	defer r.cleanupAllWorktrees()

	// Main execution loop
	for {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return r.results, err
		}

		// Get eligible tasks
		eligible := r.dag.Eligible()

		// Check if we're done
		running := r.countRunningTasks()
		if len(eligible) == 0 && running == 0 {
			// No eligible tasks and nothing running - we're done
			break
		}

		// If no eligible tasks but some are running, wait for next wave
		if len(eligible) == 0 {
			continue
		}

		// Execute wave of tasks with bounded concurrency
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(r.config.ConcurrencyLimit)

		for _, task := range eligible {
			// Capture task for closure
			t := task
			g.Go(func() error {
				return r.executeTask(gctx, t)
			})
		}

		// Wait for wave to complete
		if err := g.Wait(); err != nil {
			// Context cancellation or unrecoverable error
			if ctx.Err() != nil {
				return r.results, ctx.Err()
			}
			// Task errors are tracked in DAG, not returned here
		}
	}

	return r.results, nil
}

// countRunningTasks returns the number of tasks currently running.
func (r *ParallelRunner) countRunningTasks() int {
	count := 0
	for _, task := range r.dag.Tasks() {
		if task.Status == scheduler.TaskRunning {
			count++
		}
	}
	return count
}

// executeTask executes a single task in its own worktree.
func (r *ParallelRunner) executeTask(ctx context.Context, task *scheduler.Task) error {
	// Check context early
	if err := ctx.Err(); err != nil {
		markErr := fmt.Errorf("context cancelled before execution: %w", err)
		_ = r.dag.MarkFailed(task.ID, markErr)
		return nil // Return nil to not abort errgroup
	}

	// Mark task as running
	if err := r.dag.MarkRunning(task.ID); err != nil {
		log.Printf("ERROR: failed to mark task %q as running: %v", task.ID, err)
		return nil
	}

	// Create worktree
	wtInfo, err := r.config.WorktreeManager.Create(task.ID)
	if err != nil {
		_ = r.dag.MarkFailed(task.ID, fmt.Errorf("failed to create worktree: %w", err))
		r.recordResult(TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
		})
		return nil
	}

	// Track worktree
	r.mu.Lock()
	r.activeWorktrees[task.ID] = wtInfo
	r.mu.Unlock()

	// Ensure cleanup if we exit early
	defer func() {
		r.mu.Lock()
		delete(r.activeWorktrees, task.ID)
		r.mu.Unlock()
	}()

	// Create per-task backend
	b, err := r.createBackend(task.AgentRole, wtInfo.Path)
	if err != nil {
		_ = r.config.WorktreeManager.ForceCleanup(wtInfo)
		_ = r.dag.MarkFailed(task.ID, err)
		r.recordResult(TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
		})
		return nil
	}
	defer b.Close()

	// Acquire file locks
	r.lockMgr.LockAll(task.WritesFiles)
	defer r.lockMgr.UnlockAll(task.WritesFiles)

	// Send task to backend
	resp, err := b.Send(ctx, backend.Message{Content: task.Prompt, Role: "user"})
	if err != nil {
		_ = r.config.WorktreeManager.ForceCleanup(wtInfo)
		_ = r.dag.MarkFailed(task.ID, err)
		r.recordResult(TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
		})
		return nil
	}

	// Mark task completed
	_ = r.dag.MarkCompleted(task.ID, resp.Content)

	// Merge worktree back to main
	mergeResult, err := r.config.WorktreeManager.Merge(wtInfo, r.config.MergeStrategy)
	if err != nil {
		log.Printf("ERROR: unexpected error during merge operation for task %q: %v", task.ID, err)
		_ = r.config.WorktreeManager.ForceCleanup(wtInfo)
		r.recordResult(TaskResult{
			TaskID:      task.ID,
			Success:     false,
			MergeResult: mergeResult,
			Error:       err,
		})
		return nil
	}

	// Check merge result
	if !mergeResult.Merged {
		// Merge conflict - work succeeded but merge failed
		log.Printf("WARNING: merge conflict for task %q: %v", task.ID, mergeResult.Error)
		_ = r.config.WorktreeManager.Cleanup(wtInfo) // Keep branch for inspection
		r.recordResult(TaskResult{
			TaskID:      task.ID,
			Success:     true, // Task succeeded, merge failed
			MergeResult: mergeResult,
			Error:       mergeResult.Error,
		})
		return nil
	}

	// Merge succeeded - cleanup worktree
	if err := r.config.WorktreeManager.Cleanup(wtInfo); err != nil {
		log.Printf("WARNING: failed to cleanup worktree for task %q: %v", task.ID, err)
	}

	// Record success
	r.recordResult(TaskResult{
		TaskID:      task.ID,
		Success:     true,
		MergeResult: mergeResult,
		Error:       nil,
	})

	return nil
}

// createBackend creates a backend instance for the given agent role with worktree WorkDir.
func (r *ParallelRunner) createBackend(agentRole string, workDir string) (backend.Backend, error) {
	// Use factory if provided (for testing)
	if r.config.BackendFactory != nil {
		return r.config.BackendFactory(agentRole, workDir)
	}

	// Otherwise use BackendConfigs
	baseCfg, ok := r.config.BackendConfigs[agentRole]
	if !ok {
		return nil, fmt.Errorf("no backend config for agent role %q", agentRole)
	}

	// Clone config and set WorkDir to worktree path
	cfg := baseCfg
	cfg.WorkDir = workDir

	return backend.New(cfg, r.config.ProcessManager)
}

// recordResult appends a task result in a thread-safe manner.
func (r *ParallelRunner) recordResult(result TaskResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, result)
}

// cleanupAllWorktrees force-cleans all active worktrees.
func (r *ParallelRunner) cleanupAllWorktrees() {
	r.mu.Lock()
	worktrees := make([]*worktree.WorktreeInfo, 0, len(r.activeWorktrees))
	for _, wt := range r.activeWorktrees {
		worktrees = append(worktrees, wt)
	}
	r.mu.Unlock()

	for _, wt := range worktrees {
		if err := r.config.WorktreeManager.ForceCleanup(wt); err != nil {
			log.Printf("ERROR: failed to force cleanup worktree %q: %v", wt.TaskID, err)
		}
	}
}
