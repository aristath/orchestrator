package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aristath/orchestrator/internal/backend"
	"github.com/sony/gobreaker"
)

// retryTestBackend is a mock backend for testing retry behavior.
type retryTestBackend struct {
	mu        sync.Mutex
	responses []any // Each entry is either backend.Response or error
	callCount int
}

func (b *retryTestBackend) Send(ctx context.Context, msg backend.Message) (backend.Response, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.callCount >= len(b.responses) {
		return backend.Response{}, fmt.Errorf("unexpected call %d (only %d responses configured)", b.callCount+1, len(b.responses))
	}

	resp := b.responses[b.callCount]
	b.callCount++

	switch v := resp.(type) {
	case backend.Response:
		return v, nil
	case error:
		return backend.Response{}, v
	default:
		return backend.Response{}, fmt.Errorf("invalid response type: %T", v)
	}
}

func (b *retryTestBackend) Close() error {
	return nil
}

func (b *retryTestBackend) SessionID() string {
	return "test-session"
}

func (b *retryTestBackend) CallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.callCount
}

// TestSendWithRetry_TransientThenSuccess verifies transient failures are retried.
func TestSendWithRetry_TransientThenSuccess(t *testing.T) {
	// Backend fails twice, then succeeds
	testBackend := &retryTestBackend{
		responses: []any{
			fmt.Errorf("transient error 1"),
			fmt.Errorf("transient error 2"),
			backend.Response{Content: "success", SessionID: "test"},
		},
	}

	cb := NewCircuitBreakerRegistry().Get("test")
	retryCfg := RetryConfig{
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      1 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
	}

	ctx := context.Background()
	resp, err := sendWithRetry(ctx, testBackend, backend.Message{Content: "test"}, cb, retryCfg)

	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}

	if resp.Content != "success" {
		t.Errorf("expected response content 'success', got %q", resp.Content)
	}

	if testBackend.CallCount() != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", testBackend.CallCount())
	}
}

// TestSendWithRetry_PermanentFailure_CircuitOpen verifies circuit breaker opens after consecutive failures.
func TestSendWithRetry_PermanentFailure_CircuitOpen(t *testing.T) {
	// Backend always fails
	testBackend := &retryTestBackend{
		responses: make([]any, 20), // More than enough for circuit to open
	}
	for i := range testBackend.responses {
		testBackend.responses[i] = fmt.Errorf("persistent error %d", i+1)
	}

	cbRegistry := NewCircuitBreakerRegistry()
	cb := cbRegistry.Get("test-backend")
	retryCfg := RetryConfig{
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      500 * time.Millisecond, // Short timeout for testing
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
	}

	ctx := context.Background()

	// Make multiple requests to trip the circuit breaker
	// Circuit trips after 5 consecutive failures
	for i := range 7 {
		_, err := sendWithRetry(ctx, testBackend, backend.Message{Content: "test"}, cb, retryCfg)
		if err == nil {
			t.Errorf("call %d: expected error, got success", i+1)
		}

		// After 5th failure, circuit should be open
		if i >= 5 {
			if errors.Is(err, gobreaker.ErrOpenState) {
				// Circuit is open - this is expected
				t.Logf("call %d: circuit open (expected)", i+1)
				return // Test passed
			}
		}
	}

	// If we get here, verify circuit eventually opened
	state := cb.State()
	if state != gobreaker.StateOpen {
		t.Errorf("expected circuit to be open after 7 requests, got state: %v", state)
	}
}

// TestSendWithRetry_ContextCancelled_StopsRetry verifies context cancellation stops retries immediately.
func TestSendWithRetry_ContextCancelled_StopsRetry(t *testing.T) {
	// Backend always fails
	testBackend := &retryTestBackend{
		responses: make([]any, 100),
	}
	for i := range testBackend.responses {
		testBackend.responses[i] = fmt.Errorf("error %d", i+1)
	}

	cb := NewCircuitBreakerRegistry().Get("test")
	retryCfg := RetryConfig{
		InitialInterval:     50 * time.Millisecond,
		MaxInterval:         200 * time.Millisecond,
		MaxElapsedTime:      10 * time.Second, // Long timeout - should be interrupted by context
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := sendWithRetry(ctx, testBackend, backend.Message{Content: "test"}, cb, retryCfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded error, got: %v", err)
	}

	// Should return quickly (within 300ms), not wait for MaxElapsedTime (10s)
	if elapsed > 500*time.Millisecond {
		t.Errorf("sendWithRetry took %v, expected < 500ms (context should stop retries)", elapsed)
	}

	t.Logf("Context cancellation stopped retries after %v", elapsed)
}

// TestCircuitBreakerRegistry_PerBackendType verifies circuit breakers are per-backend-type.
func TestCircuitBreakerRegistry_PerBackendType(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	cb1a := registry.Get("claude")
	cb1b := registry.Get("claude")
	cb2 := registry.Get("codex")

	// Same backend type should return same circuit breaker instance
	if cb1a != cb1b {
		t.Error("expected same circuit breaker instance for 'claude'")
	}

	// Different backend type should return different instance
	if cb1a == cb2 {
		t.Error("expected different circuit breaker instances for 'claude' and 'codex'")
	}

	// Verify names are set correctly
	if cb1a.Name() != "claude" {
		t.Errorf("expected circuit breaker name 'claude', got %q", cb1a.Name())
	}
	if cb2.Name() != "codex" {
		t.Errorf("expected circuit breaker name 'codex', got %q", cb2.Name())
	}
}

// TestCircuitBreaker_UserCancellationNotCounted verifies user cancellation doesn't count as failure.
func TestCircuitBreaker_UserCancellationNotCounted(t *testing.T) {
	registry := NewCircuitBreakerRegistry()
	cb := registry.Get("test-backend")

	// Create backend that returns context.Canceled
	testBackend := &retryTestBackend{
		responses: []any{
			context.Canceled,
		},
	}

	retryCfg := RetryConfig{
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      100 * time.Millisecond,
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Make 5 requests with cancelled context
	// Circuit should NOT open because user cancellation is not a backend failure
	for i := range 5 {
		testBackend.mu.Lock()
		testBackend.callCount = 0 // Reset for each test
		testBackend.mu.Unlock()

		_, err := sendWithRetry(ctx, testBackend, backend.Message{Content: "test"}, cb, retryCfg)
		if err == nil {
			t.Errorf("call %d: expected error, got success", i+1)
		}
	}

	// Circuit should still be closed
	state := cb.State()
	if state != gobreaker.StateClosed {
		t.Errorf("expected circuit to remain closed after user cancellations, got state: %v", state)
	}

	t.Logf("Circuit state after 5 user cancellations: %v (expected: closed)", state)
}
