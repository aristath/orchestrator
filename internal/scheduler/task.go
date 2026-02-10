package scheduler

// TaskStatus represents the current state of a task.
type TaskStatus int

const (
	TaskPending   TaskStatus = iota // Waiting for dependencies
	TaskEligible                     // All dependencies resolved, ready to run
	TaskRunning                      // Currently executing
	TaskCompleted                    // Finished successfully
	TaskFailed                       // Finished with error
	TaskSkipped                      // Intentionally not run
)

// FailureMode determines how a task's failure affects dependents.
type FailureMode int

const (
	FailHard FailureMode = iota // Block ALL dependents
	FailSoft                     // Dependents CAN still run
	FailSkip                     // Treat as success for dependency purposes
)

// Task represents a unit of work in the DAG.
type Task struct {
	ID          string      // Unique identifier
	Name        string      // Human-readable name
	AgentRole   string      // Key into config.Agents map (e.g., "coder", "reviewer")
	Prompt      string      // The prompt/instruction for the agent
	DependsOn   []string    // Task IDs this task depends on
	WritesFiles []string    // Files this task will write (for resource locking)
	Status      TaskStatus
	FailureMode FailureMode
	Result      string      // Output from execution (populated after completion)
	Error       error       // Error if failed
}
