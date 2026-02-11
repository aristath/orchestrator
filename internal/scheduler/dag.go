package scheduler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gammazero/toposort"
)

// DAG represents a directed acyclic graph of tasks.
type DAG struct {
	mu         sync.RWMutex
	tasks      map[string]*Task    // All tasks indexed by ID
	dependents map[string][]string // Maps taskID -> list of tasks that depend on it
}

// NewDAG creates an empty DAG.
func NewDAG() *DAG {
	return &DAG{
		tasks:      make(map[string]*Task),
		dependents: make(map[string][]string),
	}
}

// AddTask adds a task to the DAG. Returns error if task ID already exists.
func (d *DAG) AddTask(task *Task) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %q already exists", task.ID)
	}

	d.tasks[task.ID] = task

	// Build dependents map for efficient downstream lookup
	for _, depID := range task.DependsOn {
		d.dependents[depID] = append(d.dependents[depID], task.ID)
	}

	return nil
}

// Validate runs topological sort using gammazero/toposort.
// Returns ordered task IDs or error if cycle detected.
// Also verifies all task IDs in DependsOn exist in the DAG.
func (d *DAG) Validate() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// First, verify all dependencies exist
	for taskID, task := range d.tasks {
		for _, depID := range task.DependsOn {
			if _, exists := d.tasks[depID]; !exists {
				return nil, fmt.Errorf("task %q depends on non-existent task %q", taskID, depID)
			}
		}
	}

	// Build edges for topological sort
	var edges []toposort.Edge
	for taskID, task := range d.tasks {
		if len(task.DependsOn) == 0 {
			// Task with no dependencies - add edge from nil to ensure it's included
			edges = append(edges, toposort.Edge{nil, taskID})
		} else {
			for _, depID := range task.DependsOn {
				// Edge (depID, taskID) means depID must come before taskID
				edges = append(edges, toposort.Edge{depID, taskID})
			}
		}
	}

	// Run topological sort
	sorted, err := toposort.Toposort(edges)
	if err != nil {
		// Try to identify cycle participants
		return nil, fmt.Errorf("DAG contains cycle: %w", err)
	}

	// Convert []interface{} to []string
	order := make([]string, 0, len(sorted))
	for _, id := range sorted {
		if id != nil {
			order = append(order, id.(string))
		}
	}

	// Verify all tasks are in the sorted result (catches disconnected components)
	if len(order) != len(d.tasks) {
		missing := []string{}
		foundMap := make(map[string]bool)
		for _, id := range order {
			foundMap[id] = true
		}
		for taskID := range d.tasks {
			if !foundMap[taskID] {
				missing = append(missing, taskID)
			}
		}
		return nil, fmt.Errorf("topological sort lost %d tasks: %s", len(missing), strings.Join(missing, ", "))
	}

	return order, nil
}

// Eligible returns all tasks with status TaskPending whose dependencies are ALL resolved.
// Does NOT include tasks already eligible/running/completed/failed.
func (d *DAG) Eligible() []*Task {
	d.mu.RLock()
	defer d.mu.RUnlock()

	eligible := []*Task{}

	for _, task := range d.tasks {
		// Only consider pending tasks
		if task.Status != TaskPending {
			continue
		}

		// Check if all dependencies are resolved
		allResolved := true
		for _, depID := range task.DependsOn {
			dep, exists := d.tasks[depID]
			if !exists {
				allResolved = false
				break
			}

			if !d.isDependencyResolved(dep) {
				allResolved = false
				break
			}
		}

		if allResolved {
			eligible = append(eligible, cloneTask(task))
		}
	}

	return eligible
}

// isDependencyResolved checks if a dependency task is resolved based on its status and failure mode.
func (d *DAG) isDependencyResolved(dep *Task) bool {
	switch dep.Status {
	case TaskCompleted:
		return true
	case TaskSkipped:
		return true
	case TaskFailed:
		// Failed tasks are resolved depending on their failure mode
		switch dep.FailureMode {
		case FailSoft:
			return true
		case FailSkip:
			return true
		case FailHard:
			return false
		}
	}
	return false
}

// MarkRunning sets task status to TaskRunning.
func (d *DAG) MarkRunning(taskID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	task, exists := d.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %q not found", taskID)
	}

	task.Status = TaskRunning
	return nil
}

// MarkCompleted sets task status to TaskCompleted and stores result.
func (d *DAG) MarkCompleted(taskID string, result string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	task, exists := d.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %q not found", taskID)
	}

	task.Status = TaskCompleted
	task.Result = result
	return nil
}

// MarkFailed sets task status to TaskFailed and stores error.
// Behavior depends on FailureMode:
// - FailHard: dependents stay pending forever
// - FailSoft: dependents can become eligible
// - FailSkip: treat as completed for dependency resolution
func (d *DAG) MarkFailed(taskID string, err error) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	task, exists := d.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %q not found", taskID)
	}

	task.Status = TaskFailed
	task.Error = err
	return nil
}

// Get returns task by ID.
func (d *DAG) Get(taskID string) (*Task, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	task, exists := d.tasks[taskID]
	if !exists {
		return nil, false
	}
	return cloneTask(task), true
}

// Tasks returns all tasks.
func (d *DAG) Tasks() []*Task {
	d.mu.RLock()
	defer d.mu.RUnlock()

	tasks := make([]*Task, 0, len(d.tasks))
	for _, task := range d.tasks {
		tasks = append(tasks, cloneTask(task))
	}
	return tasks
}

// Order returns topologically sorted task IDs (calls Validate).
func (d *DAG) Order() ([]string, error) {
	return d.Validate()
}

func cloneTask(task *Task) *Task {
	if task == nil {
		return nil
	}

	cp := *task
	if task.DependsOn != nil {
		cp.DependsOn = append([]string(nil), task.DependsOn...)
	}
	if task.WritesFiles != nil {
		cp.WritesFiles = append([]string(nil), task.WritesFiles...)
	}
	return &cp
}
