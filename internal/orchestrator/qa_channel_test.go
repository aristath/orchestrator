package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockAnswer is a test answer function that returns formatted responses
func mockAnswer(ctx context.Context, taskID, question string) (string, error) {
	return fmt.Sprintf("answer for %s: %s", taskID, question), nil
}

// TestAskAndReceive verifies basic ask-and-receive functionality
func TestAskAndReceive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	qac := NewQAChannel(10, mockAnswer)
	qac.Start(ctx)
	defer qac.Stop()

	answer, err := qac.Ask(ctx, "task1", "what is the meaning?")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}

	expected := "answer for task1: what is the meaning?"
	if answer != expected {
		t.Errorf("Expected %q, got %q", expected, answer)
	}
}

// TestMultipleConcurrentAskers verifies that multiple agents can ask questions
// concurrently without blocking each other or experiencing cross-talk
func TestMultipleConcurrentAskers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	qac := NewQAChannel(10, mockAnswer)
	qac.Start(ctx)
	defer qac.Stop()

	var wg sync.WaitGroup
	results := make(map[string]string)
	var mu sync.Mutex

	taskIDs := []string{"task1", "task2", "task3", "task4"}

	for _, taskID := range taskIDs {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			answer, err := qac.Ask(ctx, tid, "question from "+tid)
			if err != nil {
				t.Errorf("Ask from %s failed: %v", tid, err)
				return
			}
			mu.Lock()
			results[tid] = answer
			mu.Unlock()
		}(taskID)
	}

	wg.Wait()

	// Verify all 4 got answers with correct taskID routing
	if len(results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(results))
	}

	for _, taskID := range taskIDs {
		expected := fmt.Sprintf("answer for %s: question from %s", taskID, taskID)
		if results[taskID] != expected {
			t.Errorf("Task %s: expected %q, got %q", taskID, expected, results[taskID])
		}
	}
}

// TestContextCancellation_AskBlocked verifies that Ask returns promptly
// when context is cancelled while trying to send a question
func TestContextCancellation_AskBlocked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use buffer size 1
	qac := NewQAChannel(1, mockAnswer)
	qac.Start(ctx)
	defer qac.Stop()

	// Fill the buffer by sending a question without consuming it
	go qac.Ask(ctx, "blocker", "this will fill the buffer")

	// Give it time to fill the buffer
	time.Sleep(50 * time.Millisecond)

	// Now try to ask with a cancelled context
	askCtx, askCancel := context.WithCancel(context.Background())
	askCancel() // Cancel before asking

	start := time.Now()
	_, err := qac.Ask(askCtx, "task1", "should fail quickly")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected error from cancelled context, got nil")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("Ask took %v, expected < 100ms", elapsed)
	}
}

// TestContextCancellation_StopsHandler verifies that cancelling the context
// stops the handler goroutine cleanly
func TestContextCancellation_StopsHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	qac := NewQAChannel(10, mockAnswer)
	qac.Start(ctx)

	// Cancel context
	cancel()

	// Stop should return promptly as handler exits
	done := make(chan struct{})
	go func() {
		qac.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success - handler exited cleanly
	case <-time.After(1 * time.Second):
		t.Fatal("Stop did not return within 1 second")
	}
}

// TestSlowAnswer_DoesNotBlockOthers verifies that slow answers don't block
// other callers from sending questions (though answers are processed serially)
func TestSlowAnswer_DoesNotBlockOthers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	slowAnswer := func(ctx context.Context, taskID, question string) (string, error) {
		if taskID == "slow" {
			time.Sleep(200 * time.Millisecond)
		}
		return fmt.Sprintf("answer for %s", taskID), nil
	}

	qac := NewQAChannel(10, slowAnswer)
	qac.Start(ctx)
	defer qac.Stop()

	var wg sync.WaitGroup
	results := make(chan string, 2)

	// Launch slow ask
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		answer, err := qac.Ask(ctx, "slow", "slow question")
		if err != nil {
			t.Errorf("Slow ask failed: %v", err)
			return
		}
		results <- fmt.Sprintf("slow completed at %v: %s", time.Since(start), answer)
	}()

	// Give slow task time to start processing
	time.Sleep(50 * time.Millisecond)

	// Launch fast ask
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		answer, err := qac.Ask(ctx, "fast", "fast question")
		if err != nil {
			t.Errorf("Fast ask failed: %v", err)
			return
		}
		results <- fmt.Sprintf("fast completed at %v: %s", time.Since(start), answer)
	}()

	wg.Wait()
	close(results)

	// Both should complete (verifies non-blocking send)
	count := 0
	for result := range results {
		t.Log(result)
		count++
	}

	if count != 2 {
		t.Errorf("Expected 2 results, got %d", count)
	}
}

// TestAnswerError verifies that errors from the answer function are
// propagated correctly to the caller
func TestAnswerError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errorAnswer := func(ctx context.Context, taskID, question string) (string, error) {
		return "", fmt.Errorf("answer function error")
	}

	qac := NewQAChannel(10, errorAnswer)
	qac.Start(ctx)
	defer qac.Stop()

	_, err := qac.Ask(ctx, "task1", "question")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "answer function error" {
		t.Errorf("Expected 'answer function error', got %q", err.Error())
	}
}

// TestAskAfterStop verifies that asking on a cancelled context returns an error
func TestAskAfterStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	qac := NewQAChannel(10, mockAnswer)
	qac.Start(ctx)

	// Cancel and wait for stop
	cancel()
	qac.Stop()

	// Attempt to ask with cancelled context
	_, err := qac.Ask(ctx, "task1", "question after stop")
	if err == nil {
		t.Fatal("Expected error from cancelled context, got nil")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}
