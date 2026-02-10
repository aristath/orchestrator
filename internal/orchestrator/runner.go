package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/aristath/orchestrator/internal/backend"
	"github.com/aristath/orchestrator/internal/events"
	"github.com/aristath/orchestrator/internal/persistence"
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
	EventBus         *events.EventBus           // Optional event bus (nil disables event publishing)
	Store            persistence.Store          // Optional persistence store (nil disables)
}

// ParallelRunner executes DAG tasks concurrently with git worktree isolation.
type ParallelRunner struct {
	config           ParallelRunnerConfig
	dag              *scheduler.DAG
	lockMgr          *scheduler.ResourceLockManager
	mu               sync.Mutex
	mergeMu          sync.Mutex // Serializes git merge operations to prevent index.lock conflicts
	activeWorktrees  map[string]*worktree.WorktreeInfo
	results          []TaskResult
	sessions         map[string]string // Maps taskID -> sessionID for resume support
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
		sessions:        make(map[string]string),
	}
}

// publish publishes an event to the event bus if configured.
func (r *ParallelRunner) publish(topic string, event events.Event) {
	if r.config.EventBus != nil {
		r.config.EventBus.Publish(topic, event)
	}
}

// checkpoint calls the given function with the store if configured.
// Errors are logged but do not halt execution.
func (r *ParallelRunner) checkpoint(fn func(persistence.Store) error) {
	if r.config.Store != nil {
		if err := fn(r.config.Store); err != nil {
			log.Printf("WARNING: checkpoint failed: %v", err)
		}
	}
}

// Run executes all eligible tasks concurrently with bounded concurrency.
func (r *ParallelRunner) Run(ctx context.Context) ([]TaskResult, error) {
	// Persist full DAG structure to store at the start
	if r.config.Store != nil {
		for _, task := range r.dag.Tasks() {
			if err := r.config.Store.SaveTask(ctx, task); err != nil {
				log.Printf("WARNING: failed to persist task %q: %v", task.ID, err)
			}
		}
	}

	// Clean stale worktrees from prior crashes
	if err := r.config.WorktreeManager.Prune(); err != nil {
		log.Printf("WARNING: failed to prune stale worktrees: %v", err)
	}

	// Start QA channel with a dedicated context so we can stop it when Run exits
	var qaCancel context.CancelFunc
	if r.config.QAChannel != nil {
		var qaCtx context.Context
		qaCtx, qaCancel = context.WithCancel(ctx)
		r.config.QAChannel.Start(qaCtx)
		defer func() {
			qaCancel()
			r.config.QAChannel.Stop()
		}()
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

		// If no eligible tasks but some are running, wait briefly before rechecking
		if len(eligible) == 0 {
			time.Sleep(10 * time.Millisecond)
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

		// Publish progress after wave completes
		r.publishProgress()
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
	startTime := time.Now()

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

	// Checkpoint: task status changed to Running
	r.checkpoint(func(s persistence.Store) error {
		return s.UpdateTaskStatus(ctx, task.ID, scheduler.TaskRunning, "", nil)
	})

	// Publish TaskStarted event
	r.publish(events.TopicTask, events.TaskStartedEvent{
		ID:        task.ID,
		Name:      task.Name,
		AgentRole: task.AgentRole,
		Timestamp: time.Now(),
	})

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
		taskErr := err
		_ = r.dag.MarkFailed(task.ID, taskErr)

		// Checkpoint: task failed
		r.checkpoint(func(s persistence.Store) error {
			return s.UpdateTaskStatus(ctx, task.ID, scheduler.TaskFailed, "", taskErr)
		})

		// Publish TaskFailed event
		r.publish(events.TopicTask, events.TaskFailedEvent{
			ID:       task.ID,
			Err:      taskErr,
			Duration: time.Since(startTime),
			Timestamp: time.Now(),
		})

		r.recordResult(TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   taskErr,
		})
		return nil
	}

	// Mark task completed
	_ = r.dag.MarkCompleted(task.ID, resp.Content)

	// Checkpoint: save conversation, session, and completed status
	r.checkpoint(func(s persistence.Store) error {
		// Save the prompt we sent
		if err := s.SaveMessage(ctx, task.ID, "user", task.Prompt); err != nil {
			return err
		}
		// Save the response we received
		if err := s.SaveMessage(ctx, task.ID, "assistant", resp.Content); err != nil {
			return err
		}
		// Save session for resume capability
		if err := s.SaveSession(ctx, task.ID, b.SessionID(), r.backendType(task)); err != nil {
			return err
		}
		return s.UpdateTaskStatus(ctx, task.ID, scheduler.TaskCompleted, resp.Content, nil)
	})

	// Publish TaskCompleted event
	r.publish(events.TopicTask, events.TaskCompletedEvent{
		ID:       task.ID,
		Result:   resp.Content,
		Duration: time.Since(startTime),
		Timestamp: time.Now(),
	})

	// Merge worktree back to main (serialized to prevent git index.lock conflicts)
	r.mergeMu.Lock()
	mergeResult, err := r.config.WorktreeManager.Merge(wtInfo, r.config.MergeStrategy)
	r.mergeMu.Unlock()

	// Publish TaskMerged event
	r.publish(events.TopicTask, events.TaskMergedEvent{
		ID:            task.ID,
		Merged:        mergeResult != nil && mergeResult.Merged,
		ConflictFiles: func() []string {
			if mergeResult != nil {
				return mergeResult.ConflictFiles
			}
			return []string{}
		}(),
		Timestamp: time.Now(),
	})

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

	// Check if we have a persisted session for this task (extracted from workDir)
	// Note: This is for future multi-turn support; currently sessions are task-specific
	// and not reused across tasks

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

// publishProgress computes current DAG progress and publishes a DAGProgressEvent.
func (r *ParallelRunner) publishProgress() {
	tasks := r.dag.Tasks()
	var total, completed, running, failed, pending int
	total = len(tasks)

	for _, t := range tasks {
		switch t.Status {
		case scheduler.TaskCompleted:
			completed++
		case scheduler.TaskRunning:
			running++
		case scheduler.TaskFailed:
			failed++
		default:
			pending++
		}
	}

	r.publish(events.TopicDAG, events.DAGProgressEvent{
		Total:     total,
		Completed: completed,
		Running:   running,
		Failed:    failed,
		Pending:   pending,
		Timestamp: time.Now(),
	})
}

// backendType looks up the backend type from config for a given task.
// Returns "unknown" if not found.
func (r *ParallelRunner) backendType(task *scheduler.Task) string {
	if cfg, ok := r.config.BackendConfigs[task.AgentRole]; ok {
		return cfg.Type
	}
	return "unknown"
}

// Resume reconstructs the DAG from the persisted store and continues execution.
// Completed and Failed tasks are skipped; only Pending and eligible tasks are executed.
func (r *ParallelRunner) Resume(ctx context.Context) ([]TaskResult, error) {
	if r.config.Store == nil {
		return nil, fmt.Errorf("cannot resume: no Store configured")
	}

	// Load all tasks from store
	tasks, err := r.config.Store.ListTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load tasks from store: %w", err)
	}

	// Create a new DAG and add each task
	dag := scheduler.NewDAG()
	for _, task := range tasks {
		if err := dag.AddTask(task); err != nil {
			return nil, fmt.Errorf("failed to add task %q to DAG: %w", task.ID, err)
		}
	}

	// Validate DAG (cycle detection)
	if _, err := dag.Validate(); err != nil {
		return nil, fmt.Errorf("DAG validation failed: %w", err)
	}

	// Set reconstructed DAG
	r.dag = dag

	// Load persisted sessions for resume support
	for _, task := range tasks {
		sessionID, _, err := r.config.Store.GetSession(ctx, task.ID)
		if err == nil {
			r.sessions[task.ID] = sessionID
		}
		// Ignore errors - not all tasks will have sessions
	}

	// Run the DAG - eligible() will skip Completed/Failed tasks
	return r.Run(ctx)
}
