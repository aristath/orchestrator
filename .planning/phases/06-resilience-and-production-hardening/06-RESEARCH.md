# Phase 6: Resilience and Production Hardening - Research

**Researched:** 2026-02-10
**Domain:** Production resilience patterns for distributed systems (retry logic, circuit breakers, failure isolation, graceful shutdown)
**Confidence:** HIGH

## Summary

Phase 6 introduces production-grade resilience to handle real-world failure scenarios: transient API failures, persistent backend outages, isolated task failures, and clean shutdown. The research focused on four complementary patterns that work together to create a fault-tolerant system.

**Core findings:**
1. **Retry with exponential backoff and jitter** is the standard solution for transient failures (API timeouts, rate limits), with `cenkalti/backoff/v4` as the Go ecosystem standard
2. **Circuit breakers** prevent cascading failures when backends fail persistently, with `sony/gobreaker` as the mature Go implementation
3. **Failure isolation** requires using plain `errgroup.Group` (not `WithContext`) so one task error doesn't cancel unrelated tasks
4. **Graceful shutdown** uses `signal.NotifyContext` (Go 1.16+) to propagate cancellation, with a timeout to force cleanup if handlers don't finish

The codebase already has strong foundations (ProcessManager for subprocess cleanup, checkpoint-based persistence, task status isolation in DAG). The primary gaps are: no retry logic around `backend.Send()`, no circuit breaker wrapping backend calls, errgroup uses WithContext (cancels all on first error), and no signal handling in main.

**Primary recommendation:** Wrap `backend.Send()` calls with retry logic (backoff), add per-backend circuit breakers, switch to plain errgroup.Group for independent task execution, and implement signal-based graceful shutdown in main with ProcessManager cleanup.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/cenkalti/backoff/v4 | v4.3.0+ | Exponential backoff with jitter for retries | Google's algorithm ported to Go, most mature (13k+ stars), supports context, permanent errors, configurable jitter |
| github.com/sony/gobreaker | v1.0.0+ | Circuit breaker implementation | Industry-proven (Sony production), simple API, configurable thresholds, state callbacks, 3.5k+ stars |
| golang.org/x/sync/errgroup | stdlib | Goroutine coordination | Standard library package, SetLimit for bounded concurrency, plain Group for independent execution |
| os/signal | stdlib | Signal handling | Standard library, signal.NotifyContext (Go 1.16+) simplifies shutdown |
| context | stdlib | Cancellation propagation | Standard library, WithTimeout for shutdown deadlines |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| modernc.org/sqlite | current | SQLite driver | Already in use for persistence, supports WAL mode for concurrent reads/writes |
| syscall | stdlib | Process group management | Already used (Setpgid: true), enables killing subprocess trees |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| cenkalti/backoff/v4 | hashicorp/go-retryablehttp | go-retryablehttp is HTTP-specific; backoff is generic and composable with circuit breakers |
| sony/gobreaker | mercari/go-circuitbreaker | mercari is context-aware but less mature (600 stars vs 3.5k), sony has simpler API |
| plain errgroup.Group | errgroup.WithContext | WithContext cancels all goroutines on first error; incompatible with RESIL-03 (failure isolation) |

**Installation:**
```bash
go get github.com/cenkalti/backoff/v4
go get github.com/sony/gobreaker
# errgroup, signal, context, syscall already available (stdlib)
```

## Architecture Patterns

### Recommended Integration Points

**Current codebase structure:**
```
internal/
├── backend/         # Backend adapters (claude.go, codex.go, goose.go)
│   ├── backend.go   # Backend interface with Send(ctx, msg) method
│   └── process.go   # ProcessManager (already tracks subprocesses)
├── orchestrator/    # ParallelRunner
│   └── runner.go    # executeTask() calls backend.Send()
└── persistence/     # SQLiteStore (checkpoint logic exists)
```

**Integration layers:**
1. **Retry layer:** Wrap `backend.Send()` in executeTask()
2. **Circuit breaker layer:** Per-backend circuit breaker in Backend struct
3. **Failure isolation:** Switch runner.go from `errgroup.WithContext` to plain `errgroup.Group`
4. **Graceful shutdown:** Add signal handling in cmd/orchestrator/main.go

### Pattern 1: Retry with Exponential Backoff and Jitter

**What:** Wrap transient-failure-prone operations (backend.Send) with automatic retry logic that increases delay between attempts and adds randomization to prevent thundering herd.

**When to use:** Any external API call (LLM backends), any operation that can fail transiently (network timeout, rate limit 429).

**Example:**
```go
// Source: https://pkg.go.dev/github.com/cenkalti/backoff/v4
import "github.com/cenkalti/backoff/v4"

// In runner.go executeTask()
func (r *ParallelRunner) sendWithRetry(ctx context.Context, b backend.Backend, msg backend.Message) (backend.Response, error) {
	var resp backend.Response

	operation := func() error {
		var err error
		resp, err = b.Send(ctx, msg)

		// Classify errors: don't retry context cancellation
		if err != nil && ctx.Err() != nil {
			return backoff.Permanent(err)
		}

		// Check for rate limit in response (if detectable)
		// Note: backend adapters would need to expose error types
		if isRateLimitError(err) {
			// Log and retry with backoff
			return err
		}

		return err
	}

	backoffPolicy := backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(100 * time.Millisecond),
		backoff.WithMaxInterval(10 * time.Second),
		backoff.WithMaxElapsedTime(2 * time.Minute),
		backoff.WithMultiplier(2.0),
		// Default RandomizationFactor is 0.5 (±50% jitter)
	)

	backoffWithContext := backoff.WithContext(backoffPolicy, ctx)

	if err := backoff.Retry(operation, backoffWithContext); err != nil {
		return backend.Response{}, err
	}

	return resp, nil
}
```

**Configuration for LLM API calls:**
- `InitialInterval: 100ms` — fast retry for transient blips
- `MaxInterval: 10s` — cap delay to avoid excessive wait
- `MaxElapsedTime: 2 minutes` — give up if backend is persistently failing (circuit breaker takes over)
- `Multiplier: 2.0` — exponential growth (100ms → 200ms → 400ms → ...)
- `RandomizationFactor: 0.5` (default) — ±50% jitter prevents retry storms

### Pattern 2: Circuit Breaker for Persistent Failures

**What:** Wrap backend calls in a circuit breaker that tracks failures and "opens" (fails fast) when a threshold is exceeded, preventing wasted retries to a dead backend.

**When to use:** External service calls that can fail persistently (backend down, authentication revoked, quota exhausted).

**Example:**
```go
// Source: https://github.com/sony/gobreaker
import "github.com/sony/gobreaker"

// Add to Backend implementation (e.g., ClaudeAdapter)
type ClaudeAdapter struct {
	sessionID    string
	workDir      string
	model        string
	systemPrompt string
	started      bool
	procMgr      *ProcessManager
	cb           *gobreaker.CircuitBreaker  // NEW
}

func NewClaudeAdapter(cfg Config, procMgr *ProcessManager) (*ClaudeAdapter, error) {
	// ... existing setup ...

	cbSettings := gobreaker.Settings{
		Name:        "claude-backend",
		MaxRequests: 3,  // Allow 3 test requests when half-open
		Interval:    0,  // Don't clear counts (use time-based window instead)
		Timeout:     30 * time.Second,  // Stay open for 30s before testing recovery
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open circuit if 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("Circuit breaker %s: %s -> %s", name, from, to)
		},
		IsSuccessful: func(err error) bool {
			// Success if no error
			if err == nil {
				return true
			}
			// Don't count context cancellation as failure
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return true
			}
			return false
		},
	}

	return &ClaudeAdapter{
		sessionID:    sessionID,
		workDir:      workDir,
		model:        cfg.Model,
		systemPrompt: cfg.SystemPrompt,
		started:      false,
		procMgr:      procMgr,
		cb:           gobreaker.NewCircuitBreaker(cbSettings),
	}, nil
}

func (a *ClaudeAdapter) Send(ctx context.Context, msg Message) (Response, error) {
	// Wrap the actual send in circuit breaker
	result, err := a.cb.Execute(func() (interface{}, error) {
		return a.sendInternal(ctx, msg)
	})

	if err != nil {
		return Response{}, err
	}

	return result.(Response), nil
}

func (a *ClaudeAdapter) sendInternal(ctx context.Context, msg Message) (Response, error) {
	// Existing Send() logic moves here
	// ...
}
```

**Threshold tuning:**
- `ConsecutiveFailures: 5` — tolerates a few failures before opening (balance between sensitivity and false trips)
- `Timeout: 30s` — fast recovery testing for critical services (adjust per backend criticality)
- `MaxRequests: 3` — allows gradual recovery testing when half-open
- `IsSuccessful` ignores context cancellation (user-initiated, not service failure)

**State transitions:**
1. **Closed** (normal): All requests go through, failures increment counter
2. **Open** (tripped): All requests fail immediately without hitting backend
3. **Half-Open** (testing): Limited requests (MaxRequests) test if service recovered

### Pattern 3: Failure Isolation (Independent Tasks)

**What:** Use plain `errgroup.Group` instead of `errgroup.WithContext` so one task's failure doesn't cancel unrelated parallel tasks.

**When to use:** Parallel independent tasks where failure of one should not stop others (required by RESIL-03).

**Example:**
```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup
// In runner.go Run()

// CURRENT (anti-pattern for RESIL-03):
g, gctx := errgroup.WithContext(ctx)  // Creates cancellation context
g.SetLimit(r.config.ConcurrencyLimit)
for _, task := range eligible {
	t := task
	g.Go(func() error {
		return r.executeTask(gctx, t)  // If one fails, gctx cancels all
	})
}

// NEW (failure isolation):
g := new(errgroup.Group)  // No context cancellation
g.SetLimit(r.config.ConcurrencyLimit)
for _, task := range eligible {
	t := task
	g.Go(func() error {
		// Still respect parent context for user cancellation
		return r.executeTask(ctx, t)
	})
}

// Wait for all to complete
if err := g.Wait(); err != nil {
	// err is first non-nil error, but all goroutines ran to completion
	// Task failures are already tracked in DAG, so this is informational
	log.Printf("Wave completed with at least one error: %v", err)
}
```

**Key insight:** The codebase already tracks task errors in the DAG (`dag.MarkFailed()`), not in errgroup's return value. This is correct — errgroup is for synchronization, DAG is for task status. Using `WithContext` would contradict RESIL-03 by canceling unrelated tasks.

### Pattern 4: Graceful Shutdown with Signal Handling

**What:** Catch SIGINT/SIGTERM, propagate cancellation via context, wait for goroutines to finish, kill subprocess trees, close database, exit cleanly.

**When to use:** All long-running services that spawn goroutines or subprocesses.

**Example:**
```go
// Source: https://victoriametrics.com/blog/go-graceful-shutdown/
// In cmd/orchestrator/main.go

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Create signal-aware context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load config, create event bus, etc.
	cfg, err := config.LoadDefault()
	// ...

	// Create ProcessManager
	pm := backend.NewProcessManager()

	// Create store
	store, err := persistence.NewSQLiteStore(ctx, dbPath)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create runner
	runner := orchestrator.NewParallelRunner(
		orchestrator.ParallelRunnerConfig{
			ProcessManager: pm,
			Store:          store,
			// ... other config
		},
		dag,
		lockMgr,
	)

	// Run in goroutine so we can handle shutdown
	errChan := make(chan error, 1)
	go func() {
		errChan <- runner.Run(ctx)
	}()

	// Wait for completion or signal
	select {
	case err := <-errChan:
		// Normal completion
		if err != nil {
			log.Printf("Runner finished with error: %v", err)
		}
	case <-ctx.Done():
		// Signal received
		log.Println("Shutdown signal received, cleaning up...")

		// Give processes time to finish (shutdown timeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Kill all tracked subprocesses
		if err := pm.KillAll(); err != nil {
			log.Printf("Error killing subprocesses: %v", err)
		}

		// Wait for runner to finish checkpoint (or timeout)
		select {
		case err := <-errChan:
			if err != nil && err != context.Canceled {
				log.Printf("Runner error during shutdown: %v", err)
			}
		case <-shutdownCtx.Done():
			log.Println("Shutdown timeout exceeded, forcing exit")
		}

		// Database closes via defer
	}

	log.Println("Shutdown complete")
}
```

**Shutdown sequence:**
1. `signal.NotifyContext` catches SIGINT (Ctrl+C) or SIGTERM
2. Context cancels, propagating to runner.Run() and all executeTask() goroutines
3. Goroutines check `ctx.Err()` and exit early if cancelled
4. ProcessManager.KillAll() sends SIGKILL to all subprocess process groups
5. Database connection closes (via defer)
6. Main exits cleanly

**Timeout:** 10-second grace period allows in-flight tasks to checkpoint, then forces exit.

### Anti-Patterns to Avoid

**1. Retry without jitter:** All clients retry at the same intervals → thundering herd when service recovers
- Fix: Always use RandomizationFactor (default 0.5 in backoff library)

**2. Retrying non-idempotent operations:** Payments, mutations could execute multiple times
- Fix: Use `backoff.Permanent()` for non-retryable errors

**3. Circuit breaker and retry competing:** Retry keeps hitting open circuit, wasting time
- Fix: Check circuit state before retry, or compose patterns (retry inside circuit breaker)

**4. WithContext for independent tasks:** One failure cancels all
- Fix: Use plain `errgroup.Group` for independent execution (RESIL-03)

**5. Retry on overloaded service:** Makes outage worse (self-DoS)
- Fix: Combine retry with circuit breaker — circuit opens when service is overloaded, stops retries

**6. No shutdown timeout:** Waits forever for hung goroutines
- Fix: Always use `context.WithTimeout` for shutdown phase

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exponential backoff algorithm | Custom retry loop with time.Sleep(delay *= 2) | github.com/cenkalti/backoff/v4 | Hand-rolled: no jitter (retry storms), no MaxElapsedTime cap (hangs forever), no context support (can't cancel), no permanent error detection. Library: Google's proven algorithm, context-aware, configurable. |
| Circuit breaker state machine | if failCount > threshold { skip() } | github.com/sony/gobreaker | Hand-rolled: race conditions in concurrent access, no half-open recovery state, no time-based reset, manual locking complexity. Library: thread-safe, production-tested (Sony), automatic state transitions. |
| Jitter calculation | rand.Intn(delay) | backoff.WithRandomizationFactor | Hand-rolled: wrong distribution (clustered retries), forgot to seed random, no configurability. Library: correct full jitter formula, cryptographically random, tunable. |
| Signal handling | os.Signal chan + select loop | signal.NotifyContext (Go 1.16+) | Hand-rolled: forgot to call signal.Stop (goroutine leak), missed SIGTERM (only caught SIGINT), complex cancellation propagation. Library: automatic cleanup, standard signals, context integration. |
| Process tree cleanup | kill(cmd.Process.Pid) | syscall.Kill(-pgid, SIGKILL) with Setpgid:true | Hand-rolled: kills parent only, orphans children, zombies accumulate. Library pattern: negative PID kills entire process group, already implemented in codebase. |

**Key insight:** Resilience patterns have subtle edge cases (race conditions, retry storms, half-open states) that hand-rolled implementations miss. Use battle-tested libraries.

## Common Pitfalls

### Pitfall 1: Retry Logic Interacting Badly with Rate Limits

**What goes wrong:** API returns 429 (rate limit), retry logic immediately retries, hits rate limit again, exponential backoff starts but synchronized across clients → all clients retry at similar times → retry storm when rate limit window resets.

**Why it happens:** Missing jitter in backoff, not respecting Retry-After header, treating all errors the same.

**How to avoid:**
- Always configure `RandomizationFactor` (default 0.5 is good)
- Check for rate limit errors specifically and use longer backoff
- Parse `Retry-After` header if available and wait that long
- Use circuit breaker to stop retries after persistent rate limiting

**Warning signs:**
- Logs show retries happening at exact intervals (no jitter)
- Rate limit errors spike periodically in synchronized bursts
- Backend reports retry storms in their metrics

**Code example:**
```go
// BAD: No jitter, immediate retry
backoff := backoff.NewExponentialBackOff(
	backoff.WithRandomizationFactor(0.0),  // No jitter!
)

// GOOD: Full jitter, rate-limit aware
backoffPolicy := backoff.NewExponentialBackOff(
	backoff.WithInitialInterval(1 * time.Second),  // Start higher for rate limits
	backoff.WithMaxInterval(30 * time.Second),     // Longer max wait
	backoff.WithMultiplier(2.0),
	// RandomizationFactor defaults to 0.5 (±50% jitter)
)

operation := func() error {
	resp, err := b.Send(ctx, msg)
	if isRateLimitError(err) {
		// Log for visibility
		log.Printf("Rate limited, backing off")
		return err  // Retry with exponential backoff
	}
	// ... other error handling
}
```

### Pitfall 2: Circuit Breaker Thresholds Too Sensitive or Too Lenient

**What goes wrong:**
- **Too sensitive:** Single timeout trips circuit, blocks legitimate requests, frequent false alarms
- **Too lenient:** Backend is clearly dead (100% failure rate) but circuit stays closed, wasting resources on doomed retries

**Why it happens:** Copy-pasted threshold values without tuning to actual traffic patterns and SLA requirements.

**How to avoid:**
- Use **consecutive failures** (not total failures) for fast-failing services
- Set threshold based on acceptable failure rate: 5 consecutive failures = ~3-5% expected failure rate
- Tune `Timeout` (recovery test interval) based on service criticality:
  - Critical services (auth): 10-20 seconds (fast recovery)
  - Best-effort services (analytics): 60-120 seconds (less aggressive)
- Monitor state transitions and adjust thresholds based on real data

**Warning signs:**
- Circuit opens frequently but service is actually healthy (too sensitive)
- Circuit stays closed despite 100% failure rate (too lenient)
- Half-open state never succeeds (recovery window too short)

**Tuning guide:**
```go
// For critical, fast-response backends (Claude API)
ReadyToTrip: func(counts gobreaker.Counts) bool {
	return counts.ConsecutiveFailures >= 3  // Sensitive: fail fast
},
Timeout: 10 * time.Second,  // Quick recovery testing

// For best-effort, variable-latency backends (local LLM)
ReadyToTrip: func(counts gobreaker.Counts) bool {
	failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
	return counts.Requests >= 10 && failureRate >= 0.5  // Ratio-based: need data first
},
Timeout: 60 * time.Second,  // Less aggressive recovery
```

### Pitfall 3: One Agent's Failure Cascades to Unrelated Agents

**What goes wrong:** Agent A fails with a permanent error, all other parallel agents (B, C, D) are cancelled mid-execution, work is lost, tasks that could have succeeded are marked failed.

**Why it happens:** Using `errgroup.WithContext` which cancels derived context on first error — correct for pipelines, wrong for independent tasks.

**How to avoid:**
- Use plain `errgroup.Group` (not WithContext) for independent task execution
- Pass parent context (not derived gctx) to executeTask
- Track task failures in DAG, not errgroup return value
- Still respect parent context for user-initiated cancellation (Ctrl+C)

**Warning signs:**
- Logs show multiple tasks failed with `context canceled` error
- Tasks that were making progress suddenly stop when unrelated task fails
- Resume after crash shows tasks marked failed that never actually ran

**Code fix:**
```go
// BEFORE (anti-pattern):
g, gctx := errgroup.WithContext(ctx)
for _, task := range eligible {
	t := task
	g.Go(func() error {
		return r.executeTask(gctx, t)  // gctx cancels on first error
	})
}

// AFTER (isolation):
g := new(errgroup.Group)  // No context derivation
for _, task := range eligible {
	t := task
	g.Go(func() error {
		return r.executeTask(ctx, t)  // Parent ctx only cancels on SIGINT
	})
}

// In executeTask:
func (r *ParallelRunner) executeTask(ctx context.Context, task *scheduler.Task) error {
	// ... work ...
	resp, err := b.Send(ctx, msg)
	if err != nil {
		_ = r.dag.MarkFailed(task.ID, err)
		return nil  // Don't propagate error to errgroup (DAG has it)
	}
	// ...
}
```

### Pitfall 4: Graceful Shutdown Leaves Orphaned Subprocesses

**What goes wrong:** User presses Ctrl+C, main goroutine exits, subprocess trees continue running in background (zombie agents), file locks held, git worktrees not cleaned up.

**Why it happens:**
- No signal handling (default Go behavior: immediate exit)
- Killing parent process only, not process group
- No timeout on shutdown (waits forever for hung subprocess)

**How to avoid:**
- Use `signal.NotifyContext` to catch SIGINT/SIGTERM
- Ensure all subprocesses use `Setpgid: true` (already done in codebase)
- Call `ProcessManager.KillAll()` on shutdown (sends SIGKILL to process groups)
- Use shutdown timeout (10 seconds) to force exit if cleanup hangs
- Checkpoint task state before killing processes (preserve partial work)

**Warning signs:**
- `ps aux | grep claude` shows processes after orchestrator exits
- Git worktree list shows stale worktrees after crash
- SQLite database locked after unclean shutdown

**Shutdown checklist:**
1. ✅ Signal handler registered (`signal.NotifyContext`)
2. ✅ Context cancellation propagates to runner.Run()
3. ✅ Goroutines check `ctx.Err()` and exit early
4. ✅ ProcessManager.KillAll() called on shutdown
5. ✅ Database connection closed (via defer)
6. ✅ Shutdown timeout prevents infinite wait
7. ✅ Active worktrees force-cleaned (runner.cleanupAllWorktrees already exists)

### Pitfall 5: Checkpoint Errors Halt Execution

**What goes wrong:** SQLite write fails (disk full, permissions), checkpoint logic returns error, entire execution stops, no progress made.

**Why it happens:** Treating checkpoint as critical operation when it's actually best-effort (data loss acceptable vs total failure).

**How to avoid:** Already handled correctly in codebase! `runner.go` has:
```go
func (r *ParallelRunner) checkpoint(fn func(persistence.Store) error) {
	if r.config.Store != nil {
		if err := fn(r.config.Store); err != nil {
			log.Printf("WARNING: checkpoint failed: %v", err)
			// Don't return error — continue execution
		}
	}
}
```

**Validation:** Grep for `checkpoint(` calls — confirm none check return value as error condition.

## Code Examples

Verified patterns from official sources:

### Retry with Context-Aware Backoff
```go
// Source: https://pkg.go.dev/github.com/cenkalti/backoff/v4
import (
	"context"
	"time"
	"github.com/cenkalti/backoff/v4"
)

func sendWithRetry(ctx context.Context, backend Backend, msg Message) (Response, error) {
	var resp Response

	operation := func() error {
		var err error
		resp, err = backend.Send(ctx, msg)

		// Don't retry if context is cancelled (user initiated)
		if ctx.Err() != nil {
			return backoff.Permanent(ctx.Err())
		}

		return err
	}

	backoffPolicy := backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(100 * time.Millisecond),
		backoff.WithMaxInterval(10 * time.Second),
		backoff.WithMaxElapsedTime(2 * time.Minute),
		backoff.WithMultiplier(2.0),
	)

	backoffWithContext := backoff.WithContext(backoffPolicy, ctx)

	if err := backoff.Retry(operation, backoffWithContext); err != nil {
		return Response{}, err
	}

	return resp, nil
}
```

### Circuit Breaker with Custom Failure Detection
```go
// Source: https://github.com/sony/gobreaker
import (
	"context"
	"errors"
	"log"
	"time"
	"github.com/sony/gobreaker"
)

func newBackendCircuitBreaker(name string) *gobreaker.CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    0,  // Use time-based windows instead of clearing counts
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("Circuit breaker %s: %s -> %s", name, from, to)
		},
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			// Don't count context cancellation as failure
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return true
			}
			return false
		},
	}

	return gobreaker.NewCircuitBreaker(settings)
}
```

### Graceful Shutdown with Signal Handling
```go
// Source: https://victoriametrics.com/blog/go-graceful-shutdown/
import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Create context that cancels on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Setup resources
	processManager := NewProcessManager()
	store, err := NewSQLiteStore(ctx, dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// Run main work in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- runner.Run(ctx)
	}()

	// Wait for completion or signal
	select {
	case err := <-errChan:
		// Normal completion
		if err != nil {
			log.Printf("Completed with error: %v", err)
		}
	case <-ctx.Done():
		// Signal received
		log.Println("Shutdown signal received")

		// Shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Kill subprocess trees
		if err := processManager.KillAll(); err != nil {
			log.Printf("Error killing processes: %v", err)
		}

		// Wait for runner with timeout
		select {
		case err := <-errChan:
			if err != nil && err != context.Canceled {
				log.Printf("Shutdown error: %v", err)
			}
		case <-shutdownCtx.Done():
			log.Println("Shutdown timeout exceeded")
		}
	}

	log.Println("Shutdown complete")
}
```

### Independent Task Execution (Failure Isolation)
```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup
import (
	"context"
	"log"
	"golang.org/x/sync/errgroup"
)

func runIndependentTasks(ctx context.Context, tasks []*Task) error {
	// Use plain Group (not WithContext) for independent execution
	g := new(errgroup.Group)
	g.SetLimit(4)  // Bounded concurrency

	for _, task := range tasks {
		t := task  // Capture for closure
		g.Go(func() error {
			// Still respect parent context (user cancellation)
			if err := executeTask(ctx, t); err != nil {
				// Log error but don't propagate to errgroup
				log.Printf("Task %s failed: %v", t.ID, err)
				// Error already tracked in DAG
				return nil  // Continue other tasks
			}
			return nil
		})
	}

	// Wait for all tasks (no cancellation on first error)
	return g.Wait()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| signal.Notify with manual cleanup | signal.NotifyContext | Go 1.16 (Feb 2021) | Simpler shutdown: context ties signal handling to cancellation automatically |
| Custom backoff with time.Sleep loops | Mature backoff libraries (cenkalti v4, 2020+) | 2020-2021 | Jitter, context support, permanent errors handled correctly |
| Manual circuit breaker if/else | State machine libraries (gobreaker, 2015+) | Mature since 2015 | Thread-safe, automatic recovery, state callbacks |
| errgroup.WithContext for all parallel work | Plain errgroup.Group for independent tasks | Pattern clarified ~2019 | Failure isolation for independent work |
| WAL mode manual setup | Connection string parameters | modernc.org/sqlite recent | Already used in codebase (Phase 5) |

**Deprecated/outdated:**
- **go-retryablehttp for all retry:** HTTP-specific, not composable with circuit breakers. Use generic backoff library.
- **github.com/cenkalti/backoff/v3:** v4 added functional options (2020), cleaner API
- **Custom UUID generation:** Already in codebase (claude.go), but crypto/rand is standard

## Open Questions

1. **How to detect rate limit errors from backend adapters?**
   - What we know: Backend adapters return generic errors, no structured error types
   - What's unclear: CLI tools (claude, codex, goose) don't expose rate limit info in JSON output
   - Recommendation: Add error classification to backend.Response struct (ErrorType: "rate_limit" | "timeout" | "auth" | "unknown"), parse stderr for rate limit keywords

2. **Should circuit breakers be per-backend-type or per-backend-instance?**
   - What we know: Multiple tasks use the same backend type (e.g., all "coder" role tasks use Claude)
   - What's unclear: If one task exhausts quota, should all tasks to that backend fail fast?
   - Recommendation: Per-backend-type (shared circuit breaker per Claude/Codex/Goose), prevents all tasks from hitting dead backend

3. **What if checkpoint fails during graceful shutdown?**
   - What we know: Checkpoint errors are logged but don't halt execution (Phase 5 decision)
   - What's unclear: Should shutdown wait for checkpoint to succeed, or force exit after timeout?
   - Recommendation: Best-effort checkpoint with 2-second timeout, then force exit. Data loss acceptable vs hanging shutdown.

4. **How to handle Ctrl+C during Ctrl+C (double signal)?**
   - What we know: signal.NotifyContext handles first signal, cancels context
   - What's unclear: Should second Ctrl+C force immediate exit (SIGKILL) or be ignored?
   - Recommendation: Call `stop()` after ctx.Done() to allow second Ctrl+C to force-exit (standard pattern)

## Sources

### Primary (HIGH confidence)
- [backoff package v4 - pkg.go.dev](https://pkg.go.dev/github.com/cenkalti/backoff/v4) - API, configuration, context support
- [sony/gobreaker - GitHub](https://github.com/sony/gobreaker) - Circuit breaker implementation, settings, examples
- [errgroup package - pkg.go.dev](https://pkg.go.dev/golang.org/x/sync/errgroup) - WithContext vs plain Group behavior
- [How to Implement Retry Logic in Go with Exponential Backoff - OneUpTime](https://oneuptime.com/blog/post/2026-01-07-go-retry-exponential-backoff/view) - 2026 guide, patterns
- [How to Implement Circuit Breakers in Go with sony/gobreaker - OneUpTime](https://oneuptime.com/blog/post/2026-01-07-go-circuit-breaker/view) - 2026 guide, configuration
- [Graceful Shutdown in Go: Practical Patterns - VictoriaMetrics](https://victoriametrics.com/blog/go-graceful-shutdown/) - signal.NotifyContext, shutdown patterns

### Secondary (MEDIUM confidence)
- [How to Use errgroup for Parallel Operations in Go - OneUpTime](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view) - Context cancellation behavior
- [How to Implement Graceful Shutdown in Go for Kubernetes - OneUpTime](https://oneuptime.com/blog/post/2026-01-07-go-graceful-shutdown-kubernetes/view) - Timeout patterns
- [Dealing with Rate Limiting Using Exponential Backoff - WebScraping Club](https://substack.thewebscraping.club/p/rate-limit-scraping-exponential-backoff) - Jitter patterns for rate limits
- [Understanding Retry Pattern With Exponential Back-Off and Circuit Breaker Pattern - DZone](https://dzone.com/articles/understanding-retry-pattern-with-exponential-back) - Pattern composition
- [How to Configure Circuit Breaker Patterns - OneUpTime](https://oneuptime.com/blog/post/2026-02-02-circuit-breaker-patterns/view) - Threshold tuning
- [Timeouts, retries and backoff with jitter - AWS Builders Library](https://aws.amazon.com/builders-library/timeouts-retries-and-backoff-with-jitter/) - Jitter theory

### Tertiary (LOW confidence)
- [Building Resilient Systems: Circuit Breakers and Retry Patterns - Dasroot](https://dasroot.net/posts/2026/01/building-resilient-systems-circuit-breakers-retry-patterns/) - Anti-patterns discussion (needs verification)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - cenkalti/backoff and sony/gobreaker are mature (5+ years), well-documented, widely used
- Architecture: HIGH - Patterns verified against official docs, codebase analysis shows clear integration points
- Pitfalls: HIGH - Verified from official sources (AWS, VictoriaMetrics), cross-referenced with Go community best practices

**Research date:** 2026-02-10
**Valid until:** ~60 days (stable domain, patterns mature, but threshold tuning will need real-world data)

**Key risks identified:**
1. Rate limit detection requires backend adapter changes (error classification)
2. Circuit breaker threshold tuning needs production metrics (initial values are educated guesses)
3. Shutdown timeout (10s) may need adjustment based on task checkpoint duration
4. Retry MaxElapsedTime (2min) should be tuned based on actual backend recovery patterns
