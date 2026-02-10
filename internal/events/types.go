package events

import (
	"time"
)

// Event is the base interface for all events.
type Event interface {
	EventType() string
	TaskID() string
}

// Topic constants
const (
	TopicTask = "task"
	TopicDAG  = "dag"
)

// Event type constants
const (
	EventTypeTaskStarted   = "task.started"
	EventTypeTaskOutput    = "task.output"
	EventTypeTaskCompleted = "task.completed"
	EventTypeTaskFailed    = "task.failed"
	EventTypeTaskMerged    = "task.merged"
	EventTypeDAGProgress   = "dag.progress"
)

// TaskStartedEvent is published when a task begins execution.
type TaskStartedEvent struct {
	ID        string
	Name      string
	AgentRole string
	Timestamp time.Time
}

func (e TaskStartedEvent) EventType() string { return EventTypeTaskStarted }
func (e TaskStartedEvent) TaskID() string    { return e.ID }

// TaskOutputEvent is published when a task produces output.
type TaskOutputEvent struct {
	ID        string
	Line      string
	Timestamp time.Time
}

func (e TaskOutputEvent) EventType() string { return EventTypeTaskOutput }
func (e TaskOutputEvent) TaskID() string    { return e.ID }

// TaskCompletedEvent is published when a task completes successfully.
type TaskCompletedEvent struct {
	ID        string
	Result    string
	Duration  time.Duration
	Timestamp time.Time
}

func (e TaskCompletedEvent) EventType() string { return EventTypeTaskCompleted }
func (e TaskCompletedEvent) TaskID() string    { return e.ID }

// TaskFailedEvent is published when a task fails.
type TaskFailedEvent struct {
	ID        string
	Err       error
	Duration  time.Duration
	Timestamp time.Time
}

func (e TaskFailedEvent) EventType() string { return EventTypeTaskFailed }
func (e TaskFailedEvent) TaskID() string    { return e.ID }

// TaskMergedEvent is published when a task's worktree is merged.
type TaskMergedEvent struct {
	ID            string
	Merged        bool
	ConflictFiles []string
	Timestamp     time.Time
}

func (e TaskMergedEvent) EventType() string { return EventTypeTaskMerged }
func (e TaskMergedEvent) TaskID() string    { return e.ID }

// DAGProgressEvent is published when DAG progress changes.
type DAGProgressEvent struct {
	Total     int
	Completed int
	Running   int
	Failed    int
	Pending   int
	Timestamp time.Time
}

func (e DAGProgressEvent) EventType() string { return EventTypeDAGProgress }
func (e DAGProgressEvent) TaskID() string    { return "" }
