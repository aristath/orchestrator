package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
)

// newCommand creates an exec.Cmd with process group isolation.
// The Setpgid: true flag ensures the subprocess is in its own process group,
// allowing for clean termination of the entire subprocess tree.
func newCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for signal propagation
	}
	return cmd
}

// executeCommand executes a command and returns its stdout, stderr, and any error.
// This function implements the concurrent pipe reading pattern to prevent deadlocks:
// 1. Create stdout and stderr pipes
// 2. Start the command
// 3. Read both pipes concurrently in separate goroutines
// 4. Wait for both readers to complete (wg.Wait)
// 5. Wait for the command to finish (cmd.Wait)
//
// This pattern ensures pipes are fully drained before calling cmd.Wait(),
// preventing deadlocks when subprocess output exceeds pipe buffer capacity.
func executeCommand(ctx context.Context, cmd *exec.Cmd) (stdout []byte, stderr []byte, err error) {
	// Create pipes for stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Read both pipes concurrently
	var wg sync.WaitGroup
	var stdoutBuf, stderrBuf bytes.Buffer

	wg.Add(2)

	// Read stdout in a goroutine
	go func() {
		defer wg.Done()
		io.Copy(&stdoutBuf, stdoutPipe)
	}()

	// Read stderr in a goroutine
	go func() {
		defer wg.Done()
		io.Copy(&stderrBuf, stderrPipe)
	}()

	// Wait for both pipe readers to complete
	wg.Wait()

	// Now it's safe to call cmd.Wait()
	waitErr := cmd.Wait()

	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()

	// Combine wait error with stderr context if available
	if waitErr != nil {
		if len(stderr) > 0 {
			return stdout, stderr, fmt.Errorf("command failed: %w (stderr: %s)", waitErr, string(stderr))
		}
		return stdout, stderr, fmt.Errorf("command failed: %w", waitErr)
	}

	return stdout, stderr, nil
}

// killProcessGroup kills the entire process group associated with the command.
// This ensures all child processes are terminated, not just the immediate subprocess.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("process not started")
	}

	// Send SIGKILL to the entire process group (negative PID)
	// This kills all processes in the group, preventing orphaned subprocesses
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to kill process group: %w", err)
	}

	return nil
}

// ProcessManager tracks all running subprocesses and can terminate them all on shutdown.
// This prevents zombie processes and ensures clean cleanup.
//
// Usage pattern (typically in main):
//   pm := NewProcessManager()
//   ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//   defer cancel()
//   go func() {
//     <-ctx.Done()
//     pm.KillAll()
//   }()
type ProcessManager struct {
	mu    sync.Mutex
	procs map[int]*exec.Cmd
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		procs: make(map[int]*exec.Cmd),
	}
}

// Track registers a subprocess for tracking.
// Should be called after cmd.Start() when cmd.Process is available.
func (pm *ProcessManager) Track(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.procs[cmd.Process.Pid] = cmd
}

// Untrack removes a subprocess from tracking.
// Should be called after cmd.Wait() completes.
func (pm *ProcessManager) Untrack(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.procs, cmd.Process.Pid)
}

// KillAll terminates all tracked subprocesses.
// Called during shutdown to ensure clean termination.
func (pm *ProcessManager) KillAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var errs []error
	for pid, cmd := range pm.procs {
		if err := killProcessGroup(cmd); err != nil {
			errs = append(errs, fmt.Errorf("failed to kill process %d: %w", pid, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors killing processes: %v", errs)
	}

	return nil
}

// Count returns the number of currently tracked processes.
// Useful for tests and monitoring.
func (pm *ProcessManager) Count() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return len(pm.procs)
}
