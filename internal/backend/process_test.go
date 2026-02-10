package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestExecuteCommand_BasicExecution verifies basic command execution
func TestExecuteCommand_BasicExecution(t *testing.T) {
	ctx := context.Background()
	cmd := newCommand(ctx, "echo", "hello")

	stdout, stderr, err := executeCommand(ctx, cmd, nil)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(string(stdout), "hello") {
		t.Errorf("Expected stdout to contain 'hello', got: %s", stdout)
	}

	if len(stderr) > 0 {
		t.Errorf("Expected empty stderr, got: %s", stderr)
	}
}

// TestExecuteCommand_ConcurrentPipeReading_LargeOutput verifies no deadlock on large output
// This is the critical BACK-05 test — proves concurrent pipe reading prevents deadlock
func TestExecuteCommand_ConcurrentPipeReading_LargeOutput(t *testing.T) {
	ctx := context.Background()

	// Get absolute path to mock-cli.sh
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	mockCLI := filepath.Join(workDir, "../../testdata/mock-cli.sh")

	// Use a timeout to detect deadlocks
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Generate 256KB of output (well above 64KB pipe buffer)
	cmd := newCommand(ctx, "bash", mockCLI, "--large-output", "256")

	start := time.Now()
	stdout, _, err := executeCommand(ctx, cmd, nil)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v (took %v)", err, duration)
	}

	// Verify we got a substantial amount of output
	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	if len(lines) < 10000 {
		t.Errorf("Expected at least 10000 lines of output, got %d", len(lines))
	}

	// Verify it completed in reasonable time (no deadlock)
	if duration > 5*time.Second {
		t.Errorf("Command took too long (%v), possible deadlock", duration)
	}

	t.Logf("Successfully processed %d lines in %v", len(lines), duration)
}

// TestExecuteCommand_StderrCapture verifies both stdout and stderr are captured
func TestExecuteCommand_StderrCapture(t *testing.T) {
	ctx := context.Background()
	cmd := newCommand(ctx, "bash", "-c", "echo error >&2; echo ok")

	stdout, stderr, err := executeCommand(ctx, cmd, nil)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(string(stdout), "ok") {
		t.Errorf("Expected stdout to contain 'ok', got: %s", stdout)
	}

	if !strings.Contains(string(stderr), "error") {
		t.Errorf("Expected stderr to contain 'error', got: %s", stderr)
	}
}

// TestExecuteCommand_ContextCancellation verifies subprocess termination on context cancel
func TestExecuteCommand_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	mockCLI := filepath.Join(workDir, "../../testdata/mock-cli.sh")

	// Run mock-cli.sh with long sleep
	cmd := newCommand(ctx, "bash", mockCLI, "--sleep", "30")

	_, _, err = executeCommand(ctx, cmd, nil)

	// Should get an error (context deadline exceeded or killed)
	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	// Verify error is related to context or signal
	errMsg := err.Error()
	isContextError := strings.Contains(errMsg, "context deadline exceeded") ||
		strings.Contains(errMsg, "killed") ||
		strings.Contains(errMsg, "signal")

	if !isContextError {
		t.Errorf("Expected context/signal error, got: %v", err)
	}

	t.Logf("Subprocess correctly terminated: %v", err)
}

// TestProcessManager_TrackAndKillAll verifies ProcessManager tracks and terminates processes
func TestProcessManager_TrackAndKillAll(t *testing.T) {
	pm := NewProcessManager()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	mockCLI := filepath.Join(workDir, "../../testdata/mock-cli.sh")

	// Start a long-running process
	ctx := context.Background()
	cmd := newCommand(ctx, "bash", mockCLI, "--sleep", "300")

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Track it
	pm.Track(cmd)

	if pm.Count() != 1 {
		t.Errorf("Expected 1 tracked process, got %d", pm.Count())
	}

	// Kill all tracked processes
	pm.KillAll()

	// Wait for process to terminate
	err = cmd.Wait()
	if err == nil {
		t.Error("Expected process to be killed (non-nil error), got nil")
	}

	// Verify it was a signal error
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if !status.Signaled() {
				t.Errorf("Expected process to be signaled, got exit status: %v", status)
			}
		}
	}

	// Untrack after wait
	pm.Untrack(cmd)

	if pm.Count() != 0 {
		t.Errorf("Expected 0 tracked processes after Untrack, got %d", pm.Count())
	}

	t.Log("ProcessManager correctly tracked and killed subprocess")
}

// TestProcessManager_KillsProcessTree verifies process group signal propagation
// This validates BACK-06 (process group signal propagation)
func TestProcessManager_KillsProcessTree(t *testing.T) {
	pm := NewProcessManager()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	mockCLI := filepath.Join(workDir, "../../testdata/mock-cli.sh")

	// Start process that spawns a child
	ctx := context.Background()
	cmd := newCommand(ctx, "bash", mockCLI, "--spawn-child", "--sleep", "30")

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	parentPID := cmd.Process.Pid
	pm.Track(cmd)

	// Wait a moment for child to spawn
	time.Sleep(200 * time.Millisecond)

	// Kill the process tree
	pm.KillAll()

	// Wait for parent to terminate
	cmd.Wait()
	pm.Untrack(cmd)

	// Check that child processes are also gone
	// Use pgrep to check for children of the parent PID
	checkCmd := exec.Command("pgrep", "-P", fmt.Sprintf("%d", parentPID))
	output, err := checkCmd.CombinedOutput()

	// pgrep returns exit code 1 if no processes found (which is what we want)
	if err == nil && len(bytes.TrimSpace(output)) > 0 {
		t.Errorf("Child processes still running after KillAll: %s", output)
	}

	t.Logf("Process tree correctly terminated (parent PID: %d)", parentPID)
}

// TestNoZombieProcesses_StressTest validates zero zombies after 15 sequential invocations
// This validates BACK-07 and the phase success criterion
func TestNoZombieProcesses_StressTest(t *testing.T) {
	ctx := context.Background()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	mockCLI := filepath.Join(workDir, "../../testdata/mock-cli.sh")

	// Run 15 sequential subprocess invocations
	for i := 1; i <= 15; i++ {
		cmd := newCommand(ctx, "bash", mockCLI, "--echo", fmt.Sprintf("test-%d", i))

		stdout, _, err := executeCommand(ctx, cmd, nil)
		if err != nil {
			t.Fatalf("Invocation %d failed: %v", i, err)
		}

		if !strings.Contains(string(stdout), fmt.Sprintf("test-%d", i)) {
			t.Errorf("Invocation %d: unexpected output: %s", i, stdout)
		}
	}

	// Wait briefly for any cleanup
	time.Sleep(1 * time.Second)

	// Check for zombie processes
	psCmd := exec.Command("ps", "-eo", "pid,stat,command")
	output, err := psCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run ps command: %v", err)
	}

	// Look for processes in Z (zombie) state
	lines := strings.Split(string(output), "\n")
	zombies := []string{}
	for _, line := range lines {
		if strings.Contains(line, " Z") || strings.Contains(line, "Z+") {
			zombies = append(zombies, line)
		}
	}

	if len(zombies) > 0 {
		t.Errorf("Found %d zombie processes after stress test:\n%s",
			len(zombies), strings.Join(zombies, "\n"))
	}

	t.Log("Zero zombie processes after 15 sequential invocations")
}

// TestExecuteCommand_NonZeroExitCode verifies error handling and output capture on failure
func TestExecuteCommand_NonZeroExitCode(t *testing.T) {
	ctx := context.Background()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	mockCLI := filepath.Join(workDir, "../../testdata/mock-cli.sh")

	cmd := newCommand(ctx, "bash", mockCLI, "--echo", "test-output", "--exit-code", "1")

	stdout, _, err := executeCommand(ctx, cmd, nil)

	// Should get an error
	if err == nil {
		t.Fatal("Expected error due to non-zero exit code, got nil")
	}

	// But stdout and stderr should still be captured
	if !strings.Contains(string(stdout), "test-output") {
		t.Errorf("Expected stdout to be captured despite error, got: %s", stdout)
	}

	// Verify exit code in error (need to unwrap because executeCommand wraps errors)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitCode := exitErr.ExitCode(); exitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", exitCode)
		}
	} else {
		t.Errorf("Expected error to wrap *exec.ExitError, got %T: %v", err, err)
	}

	t.Logf("Correctly captured output and error: %v", err)
}
