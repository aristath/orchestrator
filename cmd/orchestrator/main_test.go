package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aristath/orchestrator/internal/backend"
)

// TestProcessManagerKillAllOnShutdown verifies that ProcessManager.KillAll()
// correctly terminates tracked processes during simulated shutdown.
func TestProcessManagerKillAllOnShutdown(t *testing.T) {
	pm := backend.NewProcessManager()

	// Start a long-running subprocess
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Process group isolation
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start subprocess: %v", err)
	}

	// Track the process
	pm.Track(cmd)

	// Verify it's tracked
	if count := pm.Count(); count != 1 {
		t.Errorf("Expected 1 tracked process, got %d", count)
	}

	// Simulate shutdown: kill all processes
	if err := pm.KillAll(); err != nil {
		t.Errorf("KillAll() failed: %v", err)
	}

	// Wait for process to terminate (should be killed)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Process terminated (expected - it was killed)
		if err == nil {
			t.Error("Expected process to be killed (non-zero exit), got nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Process did not terminate after KillAll()")
	}

	// Verify process is still tracked (KillAll doesn't untrack, that happens in executeCommand's defer)
	if count := pm.Count(); count != 1 {
		t.Errorf("Expected process to still be tracked after KillAll, got count=%d", count)
	}

	// Cleanup: untrack the process
	pm.Untrack(cmd)

	if count := pm.Count(); count != 0 {
		t.Errorf("Expected 0 tracked processes after Untrack, got %d", count)
	}
}

// TestSignalContextCancellation verifies that signal.NotifyContext produces
// a context that cancels correctly when a signal is received.
func TestSignalContextCancellation(t *testing.T) {
	// Use SIGUSR1 as a safe test signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGUSR1)
	defer stop()

	// Send SIGUSR1 to self
	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("Failed to send SIGUSR1: %v", err)
	}

	// Verify context cancels within 1 second
	select {
	case <-ctx.Done():
		// Success - context cancelled
	case <-time.After(1 * time.Second):
		t.Fatal("Context did not cancel after SIGUSR1")
	}

	// Verify context error is as expected
	if err := ctx.Err(); err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestShutdownTimeout verifies the timeout pattern works correctly.
func TestShutdownTimeout(t *testing.T) {
	// Create a context with 50ms timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Simulate waiting for a channel that never receives
	blockChan := make(chan struct{})

	start := time.Now()
	select {
	case <-blockChan:
		t.Fatal("Unexpected receive from blockChan")
	case <-ctx.Done():
		// Expected - timeout fired
		elapsed := time.Since(start)
		if elapsed < 50*time.Millisecond {
			t.Errorf("Timeout fired too early: %v", elapsed)
		}
		if elapsed > 100*time.Millisecond {
			t.Errorf("Timeout fired too late: %v", elapsed)
		}
	}

	// Verify context error is DeadlineExceeded
	if err := ctx.Err(); err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}
