package events

import (
	"sync"
)

// EventBus is a channel-based pub-sub event bus.
// Supports topic-based subscriptions and SubscribeAll for cross-topic consumption.
type EventBus struct {
	mu      sync.RWMutex
	subs    map[string][]chan Event // topic -> subscriber channels
	allSubs []chan Event            // channels subscribed to all topics
	closed  bool
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subs:    make(map[string][]chan Event),
		allSubs: make([]chan Event, 0),
	}
}

// Subscribe creates a subscription to a specific topic.
// Returns a read-only channel that receives events published to that topic.
// bufSize determines the channel buffer size (defaults to 256 if <= 0).
func (b *EventBus) Subscribe(topic string, bufSize int) <-chan Event {
	if bufSize <= 0 {
		bufSize = 256
	}

	ch := make(chan Event, bufSize)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		close(ch)
		return ch
	}

	b.subs[topic] = append(b.subs[topic], ch)

	return ch
}

// SubscribeAll creates a subscription to ALL topics.
// Returns a single read-only channel that receives events from every topic.
// bufSize determines the channel buffer size (defaults to 256 if <= 0).
func (b *EventBus) SubscribeAll(bufSize int) <-chan Event {
	if bufSize <= 0 {
		bufSize = 256
	}

	ch := make(chan Event, bufSize)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		close(ch)
		return ch
	}

	b.allSubs = append(b.allSubs, ch)

	return ch
}

// Publish sends an event to all subscribers of the given topic.
// Non-blocking: if a subscriber's channel is full, the event is dropped for that subscriber.
// Also sends to all SubscribeAll channels.
func (b *EventBus) Publish(topic string, event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Don't publish if bus is closed
	if b.closed {
		return
	}

	// Send to topic-specific subscribers
	for _, ch := range b.subs[topic] {
		select {
		case ch <- event:
		default:
			// Channel full, drop event (non-blocking)
		}
	}

	// Send to all-topic subscribers
	for _, ch := range b.allSubs {
		select {
		case ch <- event:
		default:
			// Channel full, drop event (non-blocking)
		}
	}
}

// Close closes the event bus and all subscriber channels.
// Safe to call multiple times (idempotent).
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true

	// Close all topic-specific subscribers
	for _, channels := range b.subs {
		for _, ch := range channels {
			close(ch)
		}
	}

	// Close all-topic subscribers
	for _, ch := range b.allSubs {
		close(ch)
	}
}
