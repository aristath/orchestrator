# Phase 1: Subprocess Management and Backend Abstraction - Research

**Researched:** 2026-02-10
**Domain:** Go subprocess management and CLI integration
**Confidence:** MEDIUM

## Summary

Phase 1 requires implementing a unified Backend interface that abstracts subprocess communication with three agent CLIs: Claude Code, Codex CLI, and Goose. Each CLI has different command structures, session management approaches, and JSON output formats, but all support multi-turn conversations and structured output suitable for programmatic control.

The critical technical challenge is subprocess management in Go: pipes must be read concurrently before calling `cmd.Wait()` to prevent deadlocks, process groups must be configured with `syscall.SysProcAttr{Setpgid: true}` to enable proper signal propagation, and `cmd.Wait()` must always be called to prevent zombie processes. All three CLIs support JSON or stream-json output modes, enabling structured parsing of responses.

**Primary recommendation:** Implement a Backend interface with Send/Receive methods, use adapter pattern for each CLI with CLI-specific command construction and JSON parsing, read stdout/stderr concurrently via goroutines, use process groups for signal propagation, and implement context-based cancellation with graceful shutdown.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| os/exec | stdlib | Subprocess execution | Go standard library, proven subprocess management |
| context | stdlib | Cancellation and timeouts | Standard Go pattern for lifecycle management |
| encoding/json | stdlib | JSON parsing | Built-in support for newline-delimited JSON streaming |
| syscall | stdlib | Process group management | Low-level control for signal propagation |
| os/signal | stdlib | Signal handling | Standard for graceful shutdown patterns |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| bufio.Scanner | stdlib | Line-by-line reading | Reading newline-delimited JSON from subprocess stdout |
| sync.WaitGroup | stdlib | Goroutine synchronization | Coordinating concurrent pipe readers |
| errors | stdlib | Error wrapping | Context propagation with fmt.Errorf and %w |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib os/exec | Third-party exec wrappers | No significant advantage; stdlib is well-tested and sufficient |
| encoding/json | gojay/jstream | Only needed for extreme performance; stdlib adequate for subprocess output |
| Custom signal handling | signal.NotifyContext (Go 1.16+) | NotifyContext simplifies shutdown; recommended modern approach |

**Installation:**
No external dependencies required - all standard library.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── backend/           # Backend interface and factory
│   ├── backend.go     # Interface definition
│   ├── claude.go      # Claude Code adapter
│   ├── codex.go       # Codex CLI adapter
│   ├── goose.go       # Goose adapter
│   └── process.go     # Shared subprocess utilities
├── types/             # Shared types
│   ├── message.go     # Message structures
│   └── session.go     # Session management
└── config/            # Configuration
    └── provider.go    # Provider config
```

### Pattern 1: Backend Interface with Adapter Pattern
**What:** Define a minimal Backend interface that all CLI adapters implement, encapsulating CLI-specific details behind a common abstraction.

**When to use:** When multiple backends with different protocols need to be used interchangeably.

**Example:**
```go
// Source: Go interface best practices
// https://blog.boot.dev/golang/golang-interfaces/
// https://blog.marcnuri.com/go-interfaces-design-patterns-and-best-practices

// Backend abstracts subprocess communication with agent CLIs
type Backend interface {
    // Send sends a message and returns the response
    Send(ctx context.Context, msg Message) (Response, error)

    // Close terminates the subprocess gracefully
    Close() error

    // SessionID returns the current session identifier
    SessionID() string
}

// ClaudeAdapter implements Backend for Claude Code CLI
type ClaudeAdapter struct {
    sessionID string
    workDir   string
}

func (c *ClaudeAdapter) Send(ctx context.Context, msg Message) (Response, error) {
    // Build command: claude -r <session> -p "prompt" --output-format json
    cmd := exec.CommandContext(ctx, "claude",
        "-r", c.sessionID,
        "-p", msg.Content,
        "--output-format", "json",
    )

    // Set working directory
    cmd.Dir = c.workDir

    // Configure process group for signal propagation
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    // Read output with concurrent pipe handling
    return c.executeAndParse(ctx, cmd)
}
```

### Pattern 2: Concurrent Pipe Reading to Prevent Deadlocks
**What:** Read stdout and stderr in separate goroutines before calling `cmd.Wait()` to prevent pipe buffer deadlocks.

**When to use:** Always, when using os/exec with pipes (not capturing output with CombinedOutput).

**Example:**
```go
// Source: Go os/exec documentation and GitHub issues
// https://github.com/golang/go/issues/19685
// https://medium.com/@caring_smitten_gerbil_914/running-external-programs-in-go-the-right-way-38b11d272cd1

func (a *Adapter) executeAndParse(ctx context.Context, cmd *exec.Cmd) (Response, error) {
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return Response{}, fmt.Errorf("stdout pipe: %w", err)
    }

    stderr, err := cmd.StderrPipe()
    if err != nil {
        return Response{}, fmt.Errorf("stderr pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return Response{}, fmt.Errorf("start command: %w", err)
    }

    // Read pipes concurrently BEFORE cmd.Wait()
    var wg sync.WaitGroup
    var stdoutBuf, stderrBuf bytes.Buffer
    var stdoutErr, stderrErr error

    wg.Add(2)

    // Read stdout
    go func() {
        defer wg.Done()
        _, stdoutErr = io.Copy(&stdoutBuf, stdout)
    }()

    // Read stderr
    go func() {
        defer wg.Done()
        _, stderrErr = io.Copy(&stderrBuf, stderr)
    }()

    // Wait for pipe readers to complete
    wg.Wait()

    // Now safe to call Wait
    waitErr := cmd.Wait()

    // Check all errors
    if stdoutErr != nil {
        return Response{}, fmt.Errorf("read stdout: %w", stdoutErr)
    }
    if stderrErr != nil {
        return Response{}, fmt.Errorf("read stderr: %w", stderrErr)
    }
    if waitErr != nil {
        return Response{}, fmt.Errorf("command failed: %w (stderr: %s)", waitErr, stderrBuf.String())
    }

    // Parse JSON response
    var resp Response
    if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
        return Response{}, fmt.Errorf("parse JSON: %w", err)
    }

    return resp, nil
}
```

### Pattern 3: Process Groups for Signal Propagation
**What:** Use `syscall.SysProcAttr{Setpgid: true}` to create a new process group, enabling kill of entire subprocess tree.

**When to use:** Always, to ensure Ctrl+C or SIGTERM kills all spawned processes.

**Example:**
```go
// Source: Managing Go Processes articles
// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
// https://bigkevmcd.github.io/go/pgrp/context/2019/02/19/terminating-processes-in-go.html

func newCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
    cmd := exec.CommandContext(ctx, name, args...)

    // Create new process group for signal propagation
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true, // Create new process group with PGID = PID
    }

    return cmd
}

// Kill subprocess and all children
func killProcessTree(cmd *exec.Cmd) error {
    if cmd.Process == nil {
        return nil
    }

    // Kill entire process group (negative PID = PGID)
    pgid := cmd.Process.Pid
    return syscall.Kill(-pgid, syscall.SIGKILL)
}
```

### Pattern 4: Graceful Shutdown with Context and Timeouts
**What:** Use context cancellation for graceful shutdown, with timeout escalation to SIGKILL.

**When to use:** In orchestrator main loop to handle Ctrl+C and ensure cleanup.

**Example:**
```go
// Source: Go graceful shutdown patterns
// https://victoriametrics.com/blog/go-graceful-shutdown/
// https://henvic.dev/posts/signal-notify-context/

func main() {
    // Create context that cancels on SIGINT/SIGTERM
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    // Run orchestrator
    if err := run(ctx); err != nil {
        log.Fatal(err)
    }
}

func (b *Backend) Close() error {
    // Create timeout context for graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Try graceful termination
    if b.cmd.Process != nil {
        if err := b.cmd.Process.Signal(syscall.SIGTERM); err != nil {
            return fmt.Errorf("send SIGTERM: %w", err)
        }

        // Wait for graceful exit or timeout
        done := make(chan error, 1)
        go func() {
            done <- b.cmd.Wait()
        }()

        select {
        case <-ctx.Done():
            // Timeout - force kill
            return killProcessTree(b.cmd)
        case err := <-done:
            return err
        }
    }

    return nil
}
```

### Pattern 5: Newline-Delimited JSON Streaming
**What:** Parse JSON responses line-by-line using `bufio.Scanner` and `json.Decoder` for stream-json output.

**When to use:** When CLI outputs stream-json format (Codex, Goose, Claude Code with --output-format stream-json).

**Example:**
```go
// Source: Go JSON streaming patterns
// https://utkarshkore.medium.com/the-70mb-json-that-killed-my-512mb-go-server-a-masterclass-in-stream-processing-c0b73203c336
// https://github.com/bserdar/jsonstream

func parseStreamJSON(r io.Reader) ([]Event, error) {
    scanner := bufio.NewScanner(r)
    var events []Event

    for scanner.Scan() {
        line := scanner.Bytes()
        if len(line) == 0 {
            continue
        }

        var event Event
        if err := json.Unmarshal(line, &event); err != nil {
            return nil, fmt.Errorf("parse line: %w (line: %s)", err, line)
        }

        events = append(events, event)
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("scan: %w", err)
    }

    return events, nil
}
```

### Anti-Patterns to Avoid
- **Calling Wait before reading pipes:** Causes deadlock when subprocess output exceeds pipe buffer size
- **Not using process groups:** Orphaned subprocesses continue running, consuming API credits
- **Forgetting to call Wait:** Creates zombie processes that accumulate
- **Using CombinedOutput for long-running processes:** Buffers all output in memory; use streaming instead
- **Not handling context cancellation:** Subprocess continues running after orchestrator is killed

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON parsing | Custom JSON parser | encoding/json stdlib | Handles edge cases, streaming, escaping, validation |
| Process management | Custom subprocess wrapper | os/exec + syscall | Years of battle-testing, platform differences handled |
| Signal handling | Custom signal trap | signal.NotifyContext (Go 1.16+) | Idiomatic, integrates with context cancellation |
| Concurrent pipe reading | Manual goroutine coordination | io.Copy + sync.WaitGroup pattern | Standard pattern, less error-prone |
| Error wrapping | String concatenation | fmt.Errorf with %w | Enables errors.Is/As, stack traces |

**Key insight:** Go's standard library provides robust primitives for subprocess management. The complexity lies in combining them correctly (concurrent pipe reading, process groups, signal handling), not in building custom alternatives. Use stdlib patterns and avoid reinventing these wheels.

## Common Pitfalls

### Pitfall 1: Pipe Deadlock on Large Output
**What goes wrong:** Command hangs indefinitely when subprocess output exceeds pipe buffer (typically 64KB).

**Why it happens:** Calling `cmd.Wait()` before reading stdout/stderr causes Wait to block until subprocess exits, but subprocess blocks writing to full pipe buffers, waiting for someone to read.

**How to avoid:** Always read stdout and stderr in separate goroutines BEFORE calling Wait. Use `io.Copy` or `bufio.Scanner` to consume output as subprocess produces it.

**Warning signs:** Command works fine on small inputs but hangs on larger outputs. `ps` shows subprocess in "S" (sleeping) state.

**Verification:** Test with subprocess that produces >64KB output to ensure no deadlock.

### Pitfall 2: Orphaned Processes After Ctrl+C
**What goes wrong:** Hitting Ctrl+C kills orchestrator but leaves subprocess agent CLIs running, consuming API credits.

**Why it happens:** Without `Setpgid: true`, subprocess inherits orchestrator's process group. When you kill orchestrator process, subprocess continues running as orphan.

**How to avoid:** Set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` to create new process group. On shutdown, kill entire process group with `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`.

**Warning signs:** After killing orchestrator, `ps aux | grep claude` shows agent CLI still running. API usage continues after orchestrator stops.

**Verification:** Start orchestrator, send Ctrl+C, verify no agent CLI processes remain with `ps`.

### Pitfall 3: Zombie Processes from Missed Wait
**What goes wrong:** Subprocess completes but remains as zombie ("Z" state) consuming process table entry.

**Why it happens:** When subprocess exits, kernel keeps its entry in process table until parent calls `wait()` to retrieve exit status. If parent never calls wait, zombie persists.

**How to avoid:** Always call `cmd.Wait()` after subprocess completes, even if you don't care about exit status. Use defer or ensure all code paths call Wait.

**Warning signs:** `ps` shows processes in "Z" (zombie) state. Over time, zombie count grows. Eventually system hits process limit.

**Verification:** Stress test with 10+ sequential subprocess invocations, check `ps aux | grep Z` shows no zombies.

### Pitfall 4: Race Condition Between Start and Signal
**What goes wrong:** Attempting to signal subprocess before `cmd.Start()` completes, or after process has exited.

**Why it happens:** `cmd.Process` is nil until Start returns. Similarly, process may exit before signal is sent.

**How to avoid:** Check `cmd.Process != nil` before sending signals. Handle "process not found" errors gracefully (process may have already exited).

**Warning signs:** Occasional panics on `cmd.Process.Signal()`. Errors like "no such process" in logs.

**Verification:** Test rapid start/stop cycles, signal handling during subprocess startup/shutdown.

### Pitfall 5: Incorrect JSON Parsing of Stream Output
**What goes wrong:** Attempting to parse entire stdout as single JSON object when CLI outputs newline-delimited JSON stream.

**Why it happens:** Claude Code's `stream-json` and Codex's `--json` output newline-delimited JSON (one event per line), not a single JSON array.

**How to avoid:** Use `bufio.Scanner` to read line-by-line, parse each line separately. Check CLI documentation for output format (json vs stream-json).

**Warning signs:** JSON parse errors like "invalid character '\n' after top-level value". Works with `--output-format json` but fails with `stream-json`.

**Verification:** Test parsing with actual CLI output, both json and stream-json formats.

### Pitfall 6: Context Cancellation Not Propagating to Subprocess
**What goes wrong:** Context timeout or cancellation doesn't kill subprocess, which continues running.

**Why it happens:** Using `exec.Command` instead of `exec.CommandContext`, or not checking context in subprocess coordination.

**How to avoid:** Always use `exec.CommandContext(ctx, ...)` to create commands. Context cancellation triggers subprocess kill automatically.

**Warning signs:** Timeouts don't work. Subprocess continues after context cancels.

**Verification:** Create command with timeout context, verify subprocess terminates when timeout expires.

## CLI-Specific Details

### Claude Code CLI
**Installation:** `npm install -g @anthropic/claude-code`

**Key Flags:**
- `--session-id <UUID>`: Use specific session ID (must be valid UUID)
- `-r <session-id>`, `--resume <session-id>`: Resume session by ID or name
- `-c`, `--continue`: Resume most recent session in current directory
- `-p "prompt"`, `--print`: Non-interactive mode with prompt
- `--output-format <format>`: Output format (text, json, stream-json)
- `--system-prompt <text>`: Replace entire system prompt
- `--append-system-prompt <text>`: Append to default system prompt
- `--model <model>`: Set model (e.g., claude-sonnet-4-5-20250929)
- `--add-dir <path>`: Add additional working directories

**Multi-turn conversation:**
1. First call: `claude -p "initial prompt" --output-format json --session-id <UUID>`
2. Subsequent calls: `claude -r <UUID> -p "follow-up" --output-format json`

**JSON Output Structure:**
```json
{
  "session_id": "uuid-here",
  "messages": [...],
  "result": {
    "content": [
      {"type": "text", "text": "response content"}
    ]
  }
}
```

**Sources:**
- [CLI Reference - Claude Code Docs](https://code.claude.com/docs/en/cli-reference)
- [Claude Code Developer Cheatsheet](https://awesomeclaude.ai/code-cheatsheet)

### Codex CLI
**Installation:** Follow OpenAI Codex CLI setup

**Key Flags:**
- `--json`: Output newline-delimited JSON events
- `--model <model>`: Override model (e.g., gpt-5-codex)
- `--sandbox <mode>`: Execution policy (read-only, workspace-write, danger-full-access)
- `--output-last-message <path>`: Write final message to file
- `--output-schema <path>`: JSON Schema validation

**Commands:**
- `codex exec <prompt>`: Non-interactive execution
- `codex resume [SESSION_ID]`: Resume session, use `--last` for most recent
- `codex fork [SESSION_ID]`: Branch conversation to new thread

**Multi-turn conversation:**
1. First call: `codex exec "initial prompt" --json`
2. Subsequent calls: `codex resume <THREAD_ID> --json`

**Event Format (newline-delimited JSON):**
Event types include: ThreadStarted, TurnStarted, TurnCompleted (with Usage), TurnFailed (with ThreadError), ItemStarted, ItemUpdated, ItemCompleted.

**Sources:**
- [Command line options - OpenAI Codex](https://developers.openai.com/codex/cli/reference/)
- [Codex CLI features](https://developers.openai.com/codex/cli/features/)
- [Working with OpenAI's Codex CLI](https://www.anothercodingblog.com/p/working-with-openais-codex-cli-commands)

### Goose CLI
**Installation:** Follow [Goose documentation](https://block.github.io/goose/docs/)

**Key Flags:**
- `-i <file>`, `--instructions <file>`: Instructions file (use `-` for stdin)
- `-t <text>`, `--text <text>`: Provide text directly
- `--system <text>`: Additional system instructions
- `-s`, `--interactive`: Continue in interactive mode after processing
- `-n <name>`, `--name <name>`: Name the run session
- `-r`, `--resume`: Resume from previous run
- `--output-format <format>`: Output format (text, json, stream-json)
- `--provider <provider>`: Override provider (e.g., anthropic)
- `--model <model>`: Override model (e.g., claude-4-sonnet)
- `--debug`: Output complete tool responses
- `--max-turns <n>`: Maximum turns (default: 1000)
- `--no-session`: Execute without creating session file

**Local LLM Support:**
Set provider and model for Ollama/LM Studio/llama.cpp:
```bash
goose run --provider ollama --model llama2 --text "prompt"
```

**Multi-turn conversation:**
1. First call: `goose run --text "initial" --output-format json --name <session-name>`
2. Subsequent calls: `goose run --resume --output-format json`

**JSON Output Structure:**
Exact structure depends on output-format. Use `stream-json` for newline-delimited events.

**Important Note:**
GitHub issue [#4419](https://github.com/block/goose/issues/4419) indicates `--output-format json` was a feature request as of recent checks. Verify current Goose version supports this flag. May need to parse text output or use stream-json.

**Sources:**
- [CLI Commands - Goose](https://block.github.io/goose/docs/guides/goose-cli-commands/)
- [Configure LLM Provider - Goose](https://block.github.io/goose/docs/getting-started/providers/)
- [Add --output-format json issue](https://github.com/block/goose/issues/4419)

## Code Examples

### Complete Backend Interface
```go
// Source: Synthesized from Go best practices

package backend

import (
    "context"
    "fmt"
)

// Message represents a prompt sent to the backend
type Message struct {
    Content string
    Role    string // "user" or "system"
}

// Response represents the backend's response
type Response struct {
    Content   string
    SessionID string
    Error     string // If non-empty, indicates error
}

// Backend abstracts subprocess communication with agent CLIs
type Backend interface {
    // Send sends a message and returns the response
    Send(ctx context.Context, msg Message) (Response, error)

    // Close terminates the subprocess gracefully
    Close() error

    // SessionID returns the current session identifier
    SessionID() string
}

// Config for backend creation
type Config struct {
    Type      string // "claude", "codex", "goose"
    WorkDir   string
    SessionID string
    Model     string
    Provider  string // For Goose local LLMs
}

// New creates a backend based on config
func New(cfg Config) (Backend, error) {
    switch cfg.Type {
    case "claude":
        return NewClaudeAdapter(cfg)
    case "codex":
        return NewCodexAdapter(cfg)
    case "goose":
        return NewGooseAdapter(cfg)
    default:
        return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
    }
}
```

### Zombie Process Prevention
```go
// Source: Subprocess management best practices

package backend

import (
    "context"
    "fmt"
    "os/exec"
    "sync"
)

// ProcessManager tracks running subprocesses for cleanup
type ProcessManager struct {
    mu    sync.Mutex
    procs map[int]*exec.Cmd
}

func NewProcessManager() *ProcessManager {
    return &ProcessManager{
        procs: make(map[int]*exec.Cmd),
    }
}

// Track adds a subprocess to be managed
func (pm *ProcessManager) Track(cmd *exec.Cmd) {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    if cmd.Process != nil {
        pm.procs[cmd.Process.Pid] = cmd
    }
}

// Untrack removes a subprocess after it's been waited on
func (pm *ProcessManager) Untrack(cmd *exec.Cmd) {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    if cmd.Process != nil {
        delete(pm.procs, cmd.Process.Pid)
    }
}

// CleanupAll kills all tracked subprocesses
func (pm *ProcessManager) CleanupAll() error {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    var errs []error
    for pid, cmd := range pm.procs {
        if err := killProcessTree(cmd); err != nil {
            errs = append(errs, fmt.Errorf("kill pid %d: %w", pid, err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("cleanup errors: %v", errs)
    }

    return nil
}

// WaitWithCleanup waits for cmd and ensures it's untracked
func (pm *ProcessManager) WaitWithCleanup(cmd *exec.Cmd) error {
    defer pm.Untrack(cmd)
    return cmd.Wait()
}
```

### Stress Test for Zombie Detection
```go
// Source: Testing patterns

package backend_test

import (
    "context"
    "os/exec"
    "strings"
    "testing"
    "time"
)

func TestNoZombieProcesses(t *testing.T) {
    ctx := context.Background()

    // Run 10 sequential subprocess invocations
    for i := 0; i < 10; i++ {
        cfg := backend.Config{
            Type:      "claude",
            WorkDir:   "/tmp/test",
            SessionID: fmt.Sprintf("test-%d", i),
        }

        b, err := backend.New(cfg)
        if err != nil {
            t.Fatalf("create backend: %v", err)
        }

        _, err = b.Send(ctx, backend.Message{
            Content: "echo 'test'",
            Role:    "user",
        })
        if err != nil {
            t.Fatalf("send message: %v", err)
        }

        if err := b.Close(); err != nil {
            t.Fatalf("close backend: %v", err)
        }
    }

    // Wait for potential zombies to appear
    time.Sleep(2 * time.Second)

    // Check for zombie processes
    cmd := exec.Command("ps", "aux")
    output, err := cmd.Output()
    if err != nil {
        t.Fatalf("run ps: %v", err)
    }

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if strings.Contains(line, "Z") && strings.Contains(line, "claude") {
            t.Errorf("found zombie process: %s", line)
        }
    }
}

func TestOrphanDetection(t *testing.T) {
    ctx := context.Background()

    cfg := backend.Config{
        Type:      "claude",
        WorkDir:   "/tmp/test",
        SessionID: "orphan-test",
    }

    b, err := backend.New(cfg)
    if err != nil {
        t.Fatalf("create backend: %v", err)
    }

    // Simulate orchestrator crash by not calling Close
    // In production, signal handler would call CleanupAll

    // Check no agent CLI processes remain
    time.Sleep(1 * time.Second)

    cmd := exec.Command("pgrep", "-f", "claude.*orphan-test")
    if err := cmd.Run(); err == nil {
        t.Error("orphaned claude process found after simulated crash")
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual signal handling with os/signal.Notify | signal.NotifyContext | Go 1.16 (2021) | Simplified graceful shutdown, automatic context integration |
| String concatenation for errors | fmt.Errorf with %w | Go 1.13 (2019) | Enabled errors.Is/As, proper error chains |
| Buffering entire subprocess output | Streaming with concurrent pipe reading | Always recommended | Prevents memory issues, enables progress tracking |
| Subprocess in same process group | Setpgid: true for new process group | Best practice since early Go | Prevents orphans, enables tree kill |
| Polling for subprocess completion | Context-based cancellation | Go 1.7+ (2016) | Proper timeout handling, graceful shutdown |

**Deprecated/outdated:**
- `cmd.CombinedOutput()` for long-running processes: Buffers all output in memory. Use concurrent pipe reading instead.
- `cmd.Output()` without timeout: Can hang forever. Use CommandContext with timeout.
- Manual goroutine + channel patterns for signal handling: Use signal.NotifyContext (Go 1.16+) instead.

## Open Questions

1. **Goose --output-format json availability**
   - What we know: GitHub issue [#4419](https://github.com/block/goose/issues/4419) from search results indicates it was a feature request
   - What's unclear: Current implementation status in latest Goose version
   - Recommendation: Test with actual Goose CLI to verify. If not available, parse text output or implement custom JSON wrapper

2. **Exact JSON response schemas**
   - What we know: Claude Code returns object with session_id, messages, result. Codex outputs newline-delimited events. Goose structure uncertain.
   - What's unclear: Complete schemas with all possible fields, error formats, streaming event types
   - Recommendation: Test each CLI with actual invocations, capture output, build schema from examples. Create adapter tests that validate parsing.

3. **Session ID formats and constraints**
   - What we know: Claude Code requires valid UUID format. Codex uses thread IDs. Goose uses session names or IDs.
   - What's unclear: Can we generate UUIDs ourselves for Claude Code? Are there session storage locations we need to manage?
   - Recommendation: Test session creation and resumption with generated IDs vs CLI-generated IDs. Document any filesystem state that needs cleanup.

4. **Error reporting in JSON mode**
   - What we know: Each CLI has different error handling
   - What's unclear: How do CLIs report errors in JSON output? Exit codes? Stderr? JSON error fields?
   - Recommendation: Test error scenarios (invalid prompts, API failures, timeouts) and document error detection patterns for each CLI

5. **Subprocess resource limits**
   - What we know: No limits configured currently
   - What's unclear: Should we set memory/CPU limits on subprocesses? Timeout defaults?
   - Recommendation: Start without limits, add if needed based on testing. Consider adding --max-turns flags to prevent infinite loops.

## Sources

### Primary (HIGH confidence)
- [CLI Reference - Claude Code Docs](https://code.claude.com/docs/en/cli-reference) - Complete CLI flag reference
- [Go os/exec package documentation](https://pkg.go.dev/os/exec) - Subprocess management APIs
- [Goose CLI Commands](https://block.github.io/goose/docs/guides/goose-cli-commands/) - Official command reference
- [Command line options - OpenAI Codex](https://developers.openai.com/codex/cli/reference/) - Codex CLI flags

### Secondary (MEDIUM confidence)
- [Go subprocess pipe deadlock issue #19685](https://github.com/golang/go/issues/19685) - Verified stdlib issue documenting problem
- [Killing a child process and all of its children in Go - Felix Geisendörfer](https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773) - Process group patterns
- [Graceful Shutdown in Go - VictoriaMetrics](https://victoriametrics.com/blog/go-graceful-shutdown/) - Shutdown patterns
- [Go Interfaces: Design Patterns & Best Practices - Marc Nuri](https://blog.marcnuri.com/go-interfaces-design-patterns-and-best-practices) - Interface design
- [Best Practices for Interfaces in Go - Boot.dev](https://blog.boot.dev/golang/golang-interfaces/) - Interface patterns
- [Streaming JSON in Go - Medium](https://utkarshkore.medium.com/the-70mb-json-that-killed-my-512mb-go-server-a-masterclass-in-stream-processing-c0b73203c336) - JSON streaming

### Tertiary (LOW confidence - needs validation)
- [Add --output-format json for structured output - Goose Issue #4419](https://github.com/block/goose/issues/4419) - Feature request status uncertain
- [Claude Code --output-format explanation - ClaudeLog FAQ](https://claudelog.com/faqs/what-is-output-format-in-claude-code/) - Community documentation
- Search results for CLI flags and JSON formats - Verified against official docs where possible

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go stdlib well-documented, proven patterns
- Architecture: HIGH - Adapter pattern standard for this use case, subprocess patterns documented
- CLI protocols: MEDIUM - Official docs available but need hands-on testing for exact formats
- Pitfalls: HIGH - Well-documented in Go issues and community articles
- Goose JSON support: LOW - Needs verification with actual CLI

**Research date:** 2026-02-10
**Valid until:** 2026-03-10 (30 days - Go stdlib stable, CLI APIs may evolve)
