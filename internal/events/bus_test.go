package events

import (
	"testing"
	"time"
)

// TestPublishSubscribe verifies basic publish/subscribe functionality.
func TestPublishSubscribe(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	ch := bus.Subscribe(TopicTask, 10)

	event := TaskStartedEvent{
		ID:        "task-1",
		Name:      "Test Task",
		AgentRole: "coder",
		Timestamp: time.Now(),
	}

	bus.Publish(TopicTask, event)

	select {
	case received := <-ch:
		if received.TaskID() != "task-1" {
			t.Errorf("expected task ID 'task-1', got '%s'", received.TaskID())
		}
		if received.EventType() != EventTypeTaskStarted {
			t.Errorf("expected event type '%s', got '%s'", EventTypeTaskStarted, received.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

// TestMultipleSubscribers verifies multiple subscribers receive the same event.
func TestMultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	ch1 := bus.Subscribe(TopicTask, 10)
	ch2 := bus.Subscribe(TopicTask, 10)

	event := TaskCompletedEvent{
		ID:        "task-2",
		Result:    "success",
		Duration:  100 * time.Millisecond,
		Timestamp: time.Now(),
	}

	bus.Publish(TopicTask, event)

	// Both channels should receive the event
	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case received := <-ch:
			if received.TaskID() != "task-2" {
				t.Errorf("subscriber %d: expected task ID 'task-2', got '%s'", i+1, received.TaskID())
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d: timeout waiting for event", i+1)
		}
	}
}

// TestNonBlockingSend verifies that publishing doesn't block when channels are full.
func TestNonBlockingSend(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	// Subscribe with buffer size 1
	ch := bus.Subscribe(TopicTask, 1)

	// Publish 10 events - should not deadlock
	done := make(chan bool)
	go func() {
		for i := 0; i < 10; i++ {
			event := TaskStartedEvent{
				ID:        "task-" + string(rune(i)),
				Name:      "Test",
				AgentRole: "coder",
				Timestamp: time.Now(),
			}
			bus.Publish(TopicTask, event)
		}
		done <- true
	}()

	// Publisher should complete immediately (non-blocking)
	select {
	case <-done:
		// Success - publisher didn't block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("publisher blocked (expected non-blocking behavior)")
	}

	// Verify we received at least one event (buffer size 1)
	select {
	case received := <-ch:
		if received == nil {
			t.Error("received nil event")
		}
	default:
		t.Error("expected at least one event in buffer")
	}
}

// TestCloseSignalsSubscribers verifies that closing the bus closes subscriber channels.
func TestCloseSignalsSubscribers(t *testing.T) {
	bus := NewEventBus()

	ch := bus.Subscribe(TopicTask, 10)

	// Close the bus
	bus.Close()

	// Channel should be closed (range loop should exit immediately)
	received := 0
	for range ch {
		received++
	}

	if received != 0 {
		t.Errorf("expected 0 events after close, got %d", received)
	}
}

// TestPublishAfterClose verifies publishing after close doesn't panic.
func TestPublishAfterClose(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe(TopicTask, 10)

	bus.Close()

	// This should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("publishing after close caused panic: %v", r)
		}
	}()

	event := TaskStartedEvent{
		ID:        "task-1",
		Name:      "Test",
		AgentRole: "coder",
		Timestamp: time.Now(),
	}
	bus.Publish(TopicTask, event)

	// Channel is closed, so we shouldn't receive anything
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("received event after bus was closed")
		}
	default:
		// Expected - channel closed, no data
	}
}

// TestMultipleTopics verifies topic isolation.
func TestMultipleTopics(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	taskCh := bus.Subscribe(TopicTask, 10)
	dagCh := bus.Subscribe(TopicDAG, 10)

	taskEvent := TaskStartedEvent{
		ID:        "task-1",
		Name:      "Test",
		AgentRole: "coder",
		Timestamp: time.Now(),
	}

	dagEvent := DAGProgressEvent{
		Total:     10,
		Completed: 5,
		Running:   2,
		Failed:    0,
		Pending:   3,
		Timestamp: time.Now(),
	}

	bus.Publish(TopicTask, taskEvent)
	bus.Publish(TopicDAG, dagEvent)

	// Task channel should receive task event
	select {
	case received := <-taskCh:
		if received.EventType() != EventTypeTaskStarted {
			t.Errorf("task channel: expected task event, got %s", received.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("task channel: timeout waiting for event")
	}

	// DAG channel should receive DAG event
	select {
	case received := <-dagCh:
		if received.EventType() != EventTypeDAGProgress {
			t.Errorf("dag channel: expected dag event, got %s", received.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("dag channel: timeout waiting for event")
	}

	// Task channel should NOT have DAG event
	select {
	case <-taskCh:
		t.Error("task channel received unexpected event")
	case <-time.After(10 * time.Millisecond):
		// Expected - no event
	}

	// DAG channel should NOT have task event
	select {
	case <-dagCh:
		t.Error("dag channel received unexpected event")
	case <-time.After(10 * time.Millisecond):
		// Expected - no event
	}
}

// TestSubscribeAll verifies that SubscribeAll receives events from all topics.
func TestSubscribeAll(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	allCh := bus.SubscribeAll(20)

	// Publish task event
	taskEvent := TaskStartedEvent{
		ID:        "task-1",
		Name:      "Test",
		AgentRole: "coder",
		Timestamp: time.Now(),
	}
	bus.Publish(TopicTask, taskEvent)

	// Publish DAG event
	dagEvent := DAGProgressEvent{
		Total:     10,
		Completed: 5,
		Running:   2,
		Failed:    0,
		Pending:   3,
		Timestamp: time.Now(),
	}
	bus.Publish(TopicDAG, dagEvent)

	// SubscribeAll channel should receive both events
	receivedTypes := make(map[string]bool)

	for i := 0; i < 2; i++ {
		select {
		case received := <-allCh:
			receivedTypes[received.EventType()] = true
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for event")
		}
	}

	// Verify we received both types
	if !receivedTypes[EventTypeTaskStarted] {
		t.Error("SubscribeAll did not receive task event")
	}
	if !receivedTypes[EventTypeDAGProgress] {
		t.Error("SubscribeAll did not receive DAG event")
	}

	// Should not have any more events
	select {
	case <-allCh:
		t.Error("received unexpected third event")
	case <-time.After(10 * time.Millisecond):
		// Expected - no more events
	}
}
