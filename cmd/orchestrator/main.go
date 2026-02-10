package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aristath/orchestrator/internal/events"
	"github.com/aristath/orchestrator/internal/tui"
)

func main() {
	// Create event bus
	bus := events.NewEventBus()
	defer bus.Close()

	// Create TUI model
	model := tui.New(bus)

	// Start Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Spawn a goroutine that publishes fake events for demo/testing
	go func() {
		time.Sleep(500 * time.Millisecond)

		// Simulate task-1 starting
		bus.Publish(events.TopicTask, events.TaskStartedEvent{
			ID: "task-1", Name: "Implement auth", AgentRole: "coder",
			Timestamp: time.Now(),
		})

		// Simulate some output
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			bus.Publish(events.TopicTask, events.TaskOutputEvent{
				ID: "task-1", Line: fmt.Sprintf("[task-1] Working on auth... step %d", i+1),
				Timestamp: time.Now(),
			})
		}

		// Simulate task-2 starting
		bus.Publish(events.TopicTask, events.TaskStartedEvent{
			ID: "task-2", Name: "Write tests", AgentRole: "tester",
			Timestamp: time.Now(),
		})

		// DAG progress
		bus.Publish(events.TopicDAG, events.DAGProgressEvent{
			Total: 5, Completed: 0, Running: 2, Failed: 0, Pending: 3,
			Timestamp: time.Now(),
		})

		// More output interleaved
		for i := 0; i < 10; i++ {
			time.Sleep(150 * time.Millisecond)
			bus.Publish(events.TopicTask, events.TaskOutputEvent{
				ID: "task-2", Line: fmt.Sprintf("[task-2] Running test suite... %d/10", i+1),
				Timestamp: time.Now(),
			})
			bus.Publish(events.TopicTask, events.TaskOutputEvent{
				ID: "task-1", Line: fmt.Sprintf("[task-1] Auth implementation continued... %d", i+21),
				Timestamp: time.Now(),
			})
		}

		// Complete task-1
		bus.Publish(events.TopicTask, events.TaskCompletedEvent{
			ID: "task-1", Result: "Auth implemented successfully",
			Duration: 5 * time.Second, Timestamp: time.Now(),
		})

		bus.Publish(events.TopicDAG, events.DAGProgressEvent{
			Total: 5, Completed: 1, Running: 1, Failed: 0, Pending: 3,
			Timestamp: time.Now(),
		})

		// Complete task-2
		time.Sleep(1 * time.Second)
		bus.Publish(events.TopicTask, events.TaskCompletedEvent{
			ID: "task-2", Result: "All tests passed",
			Duration: 3 * time.Second, Timestamp: time.Now(),
		})

		bus.Publish(events.TopicDAG, events.DAGProgressEvent{
			Total: 5, Completed: 2, Running: 0, Failed: 0, Pending: 3,
			Timestamp: time.Now(),
		})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
