# Phase 3: Parallel Execution with Git Isolation - Research

**Researched:** 2026-02-10
**Domain:** Concurrent execution with git worktrees, Go errgroup, and inter-process communication
**Confidence:** HIGH

## Summary

Phase 3 requires orchestrating 2-4 concurrent agent executions using git worktrees for file isolation, bounded concurrency via errgroup.SetLimit, automatic merge strategies for integrating results, and a non-blocking communication channel for orchestrator Q&A. This research identifies the standard stack, proven architectural patterns, and critical pitfalls specific to parallel AI agent execution with git isolation.

**Primary recommendation:** Use native `git worktree` commands via os/exec (not go-git library) for worktree lifecycle management, golang.org/x/sync/errgroup for bounded concurrency with SetLimit, and buffered channels with select statements for non-blocking orchestrator-worker communication. Implement automatic cleanup on both success and failure paths to prevent orphaned worktrees and branches.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/sync/errgroup | v0.9.0+ | Bounded concurrent execution with error propagation | Official Go extended library, SetLimit provides exact bounded concurrency control, automatic context cancellation on first error |
| os/exec | stdlib | Execute git worktree commands | Native Go subprocess execution, simpler than go-git for git CLI operations, proven in production AI agent systems |
| context | stdlib | Cancellation propagation and timeouts | Standard Go pattern for graceful shutdown and timeout management across goroutines |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/ldez/go-git-cmd-wrapper | Latest | Type-safe git command builder | Optional: when you want fluent API instead of raw exec.Command, adds safety for complex git commands |
| bufio.Scanner | stdlib | Parse git command output | Line-by-line output processing from `git worktree list --porcelain`, `git status`, etc. |
| os/signal | stdlib | Graceful shutdown on SIGTERM/SIGINT | Ensures worktree cleanup on orchestrator termination |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| os/exec git commands | go-git/go-git v5 | go-git is pure Go implementation but incomplete worktree support, lacks `git worktree prune`, and doesn't match git CLI behavior exactly. Use go-git only if you need to avoid git dependency entirely. |
| errgroup.SetLimit | semaphore.Weighted | Semaphore requires manual acquire/release, errgroup integrates context cancellation and error propagation automatically. Use semaphore only for non-error-based resource limiting. |
| Buffered channels | Unbuffered channels with goroutines | Unbuffered requires extra goroutine per communication, buffered allows non-blocking sends up to capacity. Use unbuffered only when you need guaranteed synchronization. |

**Installation:**
```bash
go get golang.org/x/sync/errgroup@latest
# os/exec, context, bufio, os/signal are in stdlib
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── worktree/           # Git worktree lifecycle management
│   ├── manager.go      # Create, cleanup, merge operations
│   ├── types.go        # WorktreeInfo, MergeStrategy types
│   └── manager_test.go
├── orchestrator/       # Parallel execution orchestrator
│   ├── runner.go       # errgroup-based parallel executor
│   ├── qa_channel.go   # Question/answer communication
│   └── runner_test.go
└── scheduler/          # Existing: DAG, executor, task types
    └── executor.go     # Extend for worktree integration
```

### Pattern 1: Bounded Concurrency with errgroup.SetLimit
**What:** Limit concurrent agent executions to a configurable maximum (e.g., 2-4) using errgroup.SetLimit, ensuring all goroutines complete and propagating the first error encountered.

**When to use:** When you need to control resource usage (CPU, memory, file descriptors) while executing multiple independent tasks in parallel.

**Example:**
```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup
import (
    "context"
    "golang.org/x/sync/errgroup"
)

type ParallelExecutor struct {
    concurrencyLimit int
}

func (pe *ParallelExecutor) ExecuteTasksInParallel(ctx context.Context, tasks []*Task) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(pe.concurrencyLimit) // CRITICAL: Set before calling Go()

    for _, task := range tasks {
        task := task // Capture for closure
        g.Go(func() error {
            // Check context before expensive operations
            if err := ctx.Err(); err != nil {
                return err
            }
            return pe.executeInWorktree(ctx, task)
        })
    }

    // Wait blocks until all goroutines complete
    // Returns first non-nil error (if any)
    return g.Wait()
}
```

**Critical constraints:**
- SetLimit MUST be called before any Go() calls
- Limit cannot be modified while goroutines are active
- Go() blocks when at capacity (use TryGo() for non-blocking)
- First error cancels context and is returned by Wait()

### Pattern 2: Git Worktree Lifecycle Management
**What:** Programmatically create isolated git worktrees for each agent, execute work, merge results back, and clean up both worktree and branch.

**When to use:** When multiple agents must modify files concurrently without conflicts, sharing the same git repository but operating in separate working directories.

**Example:**
```go
// Source: https://git-scm.com/docs/git-worktree
import (
    "fmt"
    "os/exec"
    "path/filepath"
)

type WorktreeManager struct {
    repoPath string
    baseBranch string
}

// CreateWorktree creates an isolated worktree with a new branch
func (wm *WorktreeManager) CreateWorktree(taskID string) (worktreePath string, branchName string, err error) {
    branchName = fmt.Sprintf("task/%s", taskID)
    worktreePath = filepath.Join(wm.repoPath, ".worktrees", taskID)

    // git worktree add -b <branch> <path> <base-branch>
    cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, wm.baseBranch)
    cmd.Dir = wm.repoPath

    if output, err := cmd.CombinedOutput(); err != nil {
        return "", "", fmt.Errorf("worktree add failed: %w\n%s", err, output)
    }

    return worktreePath, branchName, nil
}

// MergeAndCleanup merges the worktree branch back and removes worktree+branch
func (wm *WorktreeManager) MergeAndCleanup(worktreePath, branchName string, strategy MergeStrategy) error {
    // Switch to base branch in main worktree
    if err := wm.checkout(wm.baseBranch); err != nil {
        return fmt.Errorf("checkout base failed: %w", err)
    }

    // Merge with strategy
    mergeCmd := exec.Command("git", "merge", "-s", strategy.String(), branchName)
    mergeCmd.Dir = wm.repoPath
    if output, err := mergeCmd.CombinedOutput(); err != nil {
        return fmt.Errorf("merge failed: %w\n%s", err, output)
    }

    // Remove worktree
    removeCmd := exec.Command("git", "worktree", "remove", worktreePath)
    removeCmd.Dir = wm.repoPath
    if output, err := removeCmd.CombinedOutput(); err != nil {
        // Worktree might be dirty, force if necessary
        forceCmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
        forceCmd.Dir = wm.repoPath
        if forceOutput, forceErr := forceCmd.CombinedOutput(); forceErr != nil {
            return fmt.Errorf("worktree remove failed: %w\n%s\n%s", forceErr, output, forceOutput)
        }
    }

    // Delete branch
    branchCmd := exec.Command("git", "branch", "-d", branchName)
    branchCmd.Dir = wm.repoPath
    if output, err := branchCmd.CombinedOutput(); err != nil {
        // Try force delete if regular delete fails
        forceCmd := exec.Command("git", "branch", "-D", branchName)
        forceCmd.Dir = wm.repoPath
        if forceOutput, forceErr := forceCmd.CombinedOutput(); forceErr != nil {
            return fmt.Errorf("branch delete failed: %w\n%s\n%s", forceErr, output, forceOutput)
        }
    }

    return nil
}
```

**Critical operations:**
1. `git worktree add -b <branch> <path> <base>` - Creates worktree + new branch
2. Agent executes in worktree directory (subprocess with WorkDir set)
3. `git merge -s <strategy> <branch>` - Merge back to base branch
4. `git worktree remove <path>` - Remove worktree
5. `git branch -d <branch>` - Delete branch (prevents orphans)

### Pattern 3: Non-Blocking Orchestrator Q&A Channel
**What:** Buffered channel allowing satellite agents to ask orchestrator questions without blocking other agents, with select statement for non-blocking sends/receives.

**When to use:** When worker goroutines need to communicate with orchestrator for clarifications while other workers continue executing independently.

**Example:**
```go
// Source: https://gobyexample.com/worker-pools and concurrency patterns
type Question struct {
    TaskID   string
    Question string
    Response chan string // Response channel for this specific question
}

type Orchestrator struct {
    planContext  string // Full plan context
    questionChan chan Question
}

func (o *Orchestrator) Start(ctx context.Context) {
    // Buffer size prevents blocking up to N questions
    o.questionChan = make(chan Question, 10)

    go o.handleQuestions(ctx)
}

func (o *Orchestrator) handleQuestions(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case q := <-o.questionChan:
            // Answer using plan context (non-blocking to other workers)
            answer := o.answerQuestion(q.Question, o.planContext)

            // Send answer back to worker via response channel
            select {
            case q.Response <- answer:
            case <-ctx.Done():
                return
            }
        }
    }
}

// AskOrchestrator allows workers to ask questions without blocking
func (o *Orchestrator) AskOrchestrator(ctx context.Context, taskID, question string) (string, error) {
    responseChan := make(chan string, 1)

    q := Question{
        TaskID:   taskID,
        Question: question,
        Response: responseChan,
    }

    select {
    case o.questionChan <- q:
        // Question sent, wait for response
        select {
        case answer := <-responseChan:
            return answer, nil
        case <-ctx.Done():
            return "", ctx.Err()
        }
    case <-ctx.Done():
        return "", ctx.Err()
    }
}
```

**Key design choices:**
- Buffered question channel (size 10+) prevents blocking when orchestrator is answering
- Per-question response channel ensures answer goes to correct worker
- Double select pattern: outer for send, inner for receive with context cancellation
- Orchestrator goroutine dedicated to answering (doesn't block parallel execution)

### Pattern 4: Graceful Shutdown with Cleanup
**What:** Listen for OS signals (SIGTERM, SIGINT), cancel context, wait for errgroup completion, clean up all worktrees.

**When to use:** Always - prevents orphaned worktrees and dirty git state when orchestrator is interrupted.

**Example:**
```go
// Source: https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/
import (
    "context"
    "os"
    "os/signal"
    "syscall"
)

func (o *Orchestrator) Run(ctx context.Context, tasks []*Task) error {
    // Create cancellable context
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

    // Cleanup tracker
    worktrees := make(map[string]WorktreeInfo)
    defer func() {
        // Always cleanup worktrees on exit
        for _, wt := range worktrees {
            _ = o.wtManager.ForceCleanup(wt.Path, wt.Branch)
        }
    }()

    // Listen for signals in background
    go func() {
        select {
        case <-sigChan:
            cancel() // Cancel context, triggering shutdown
        case <-ctx.Done():
        }
    }()

    // Execute with bounded concurrency
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(o.concurrencyLimit)

    for _, task := range tasks {
        task := task
        g.Go(func() error {
            // Create worktree
            wt, err := o.wtManager.CreateWorktree(task.ID)
            if err != nil {
                return err
            }

            // Track for cleanup
            worktrees[task.ID] = wt

            // Execute task
            err = o.executeInWorktree(ctx, task, wt.Path)

            // Merge or cleanup on success/failure
            if err == nil {
                return o.wtManager.MergeAndCleanup(wt.Path, wt.Branch, o.mergeStrategy)
            }
            return o.wtManager.ForceCleanup(wt.Path, wt.Branch)
        })
    }

    return g.Wait()
}
```

### Anti-Patterns to Avoid

- **Unlimited concurrency:** Never use errgroup without SetLimit - will spawn goroutines for all tasks, exhausting resources
- **Manual worktree deletion:** Never `rm -rf` worktree directories - always use `git worktree remove` to update git metadata
- **Blocking question channels:** Never use unbuffered question channel without separate goroutine per worker - will deadlock when orchestrator is busy
- **Ignoring context cancellation:** Never execute expensive operations without checking ctx.Err() - prevents timely shutdown
- **Forgetting branch cleanup:** Never remove worktree without deleting branch - leaves orphaned branches cluttering `git branch` output

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Goroutine pool with error handling | Custom worker pool with error channels | errgroup.WithContext + SetLimit | errgroup handles error propagation, context cancellation, WaitGroup semantics, and bounded concurrency automatically |
| Git worktree parsing | Custom `git worktree list` parser | git worktree list --porcelain | Porcelain format is machine-readable, stable across git versions, NUL-terminated option for safe parsing |
| Merge conflict detection | Custom diff-based conflict checker | git merge-tree --write-tree (dry-run merge) | git merge-tree detects conflicts without modifying working tree, handles all edge cases (renames, submodules, binary files) |
| Process output streaming | Manual bufio with custom line splitting | bufio.Scanner with ScanLines | Scanner handles partial reads, buffering, and various line endings automatically |
| Graceful shutdown coordination | Custom shutdown manager with done channels | context.WithCancel + signal.NotifyContext | Standard pattern, integrates with errgroup, automatic cleanup propagation |

**Key insight:** Git worktree and errgroup have subtle edge cases (shared refs, rename detection, context propagation timing) that appear simple but hide complexity. Community tools like clash-sh/clash and Worktrunk exist specifically to handle these pitfalls - if building custom, study their implementation.

## Common Pitfalls

### Pitfall 1: Branch Checkout Exclusivity
**What goes wrong:** Attempting to check out the same branch in multiple worktrees fails with "already checked out" error, blocking parallel execution.

**Why it happens:** Git enforces single checkout per branch to prevent HEAD conflicts. Each worktree must use a unique branch.

**How to avoid:** Always create new branch per worktree: `git worktree add -b task/<uuid> <path> <base-branch>`. Never use `git worktree add <path> <existing-branch>` for parallel tasks.

**Warning signs:** Error message "fatal: 'branch-name' is already checked out at '/other/path'". Worktree creation fails intermittently when tasks reuse IDs.

### Pitfall 2: Orphaned Worktrees After Failure
**What goes wrong:** Agent crashes or context cancels before cleanup, leaving worktree directory and branch. Over time, hundreds of orphaned branches accumulate.

**Why it happens:** Cleanup code in defer or happy path only. No cleanup on panic, SIGKILL, or errgroup early exit.

**How to avoid:** Track all created worktrees in map, use `defer` to cleanup regardless of outcome. Implement `ForceCleanup` that uses `--force` flags:
```go
defer func() {
    for _, wt := range activeWorktrees {
        _ = exec.Command("git", "worktree", "remove", "--force", wt.Path).Run()
        _ = exec.Command("git", "branch", "-D", wt.Branch).Run()
    }
}()
```

**Warning signs:** `git worktree list` shows many worktrees. `git branch` output is pages long. Disk usage grows over time in `.git/worktrees/`.

### Pitfall 3: SetLimit Called After Go()
**What goes wrong:** Concurrency limit is ignored, all goroutines spawn immediately, exhausting resources.

**Why it happens:** errgroup.SetLimit has no effect if called after Go(). Documentation states "limit must not be modified while goroutines are active" but doesn't error if called late.

**How to avoid:** Always call SetLimit immediately after errgroup.WithContext, before loop:
```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(4) // MUST be here, not inside loop
for _, task := range tasks {
    g.Go(func() error { ... })
}
```

**Warning signs:** All tasks start simultaneously despite SetLimit(4). Memory usage spikes when processing 100+ tasks. No blocking observed at Go() call.

### Pitfall 4: Shared Git Config and Hooks
**What goes wrong:** Git hooks (pre-commit, post-merge) run in all worktrees, sometimes with wrong context. Config changes in one worktree affect others.

**Why it happens:** Most git config and all hooks are shared across worktrees by default. Only HEAD, index, and some refs are per-worktree.

**How to avoid:**
- Use `git config extensions.worktreeConfig true` then `git config --worktree <key> <value>` for per-worktree settings
- Skip hooks in CI/automated contexts: `git commit --no-verify`
- Document which hooks are safe for parallel execution

**Warning signs:** Pre-commit hooks fail in worktree but not main. Config changes mysteriously affect other tasks. Hooks reference files not present in worktree.

### Pitfall 5: Merge Conflicts Block Entire Workflow
**What goes wrong:** One agent's changes conflict with base branch or previous merge, stopping all remaining merges and leaving worktrees unmerged.

**Why it happens:** Sequential merge-back without conflict detection. Default merge fails on conflict, doesn't continue to next task.

**How to avoid:**
1. Use `git merge-tree --write-tree` to detect conflicts before merge
2. On conflict: skip merge, preserve worktree, notify user, continue other tasks
3. Implement merge strategy configuration (ort, ours, theirs, manual)
4. Consider merging all worktrees to temporary branch, then one final merge to main

**Warning signs:** errgroup returns merge conflict error, but other tasks succeeded. Worktrees left in limbo. Git state is dirty after orchestrator exit.

### Pitfall 6: Context Cancellation Not Propagated to Subprocess
**What goes wrong:** Context cancels but subprocess agents continue running, consuming resources and modifying files after timeout.

**Why it happens:** exec.Command requires explicit CommandContext() to respect context cancellation. Using Command() ignores context.

**How to avoid:**
```go
// WRONG: subprocess ignores context
cmd := exec.Command("claude", "code")
cmd.Run()

// RIGHT: subprocess killed on context cancel
cmd := exec.CommandContext(ctx, "claude", "code")
cmd.Run() // Returns when ctx cancels or process exits
```

**Warning signs:** Processes remain after orchestrator exits. `ps aux | grep claude` shows orphans. Timeout errors but tasks complete later.

### Pitfall 7: Forgetting git worktree prune
**What goes wrong:** Manual deletion of worktree directories (e.g., during debugging) leaves stale metadata in `.git/worktrees/`, causing "worktree already exists" errors on reuse.

**Why it happens:** Git tracks worktrees in metadata separate from filesystem. Deleting directory doesn't update metadata.

**How to avoid:**
- Always use `git worktree remove <path>`, never `rm -rf`
- On startup, run `git worktree prune` to clean stale entries
- Implement health check that compares `git worktree list` with filesystem

**Warning signs:** Error "worktree '/path/to/worktree' already exists" but directory doesn't exist. `.git/worktrees/` contains entries with missing `gitdir` targets.

## Code Examples

Verified patterns from official sources:

### Bounded Concurrency with Early Cancellation
```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup
func ProcessTasksWithBoundedConcurrency(ctx context.Context, tasks []*Task, limit int) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(limit)

    for _, task := range tasks {
        task := task // Capture for closure
        g.Go(func() error {
            // Context is cancelled if any goroutine returns error
            select {
            case <-ctx.Done():
                return ctx.Err()
            default:
            }

            return processTask(ctx, task)
        })
    }

    // First error cancels context and is returned here
    return g.Wait()
}
```

### Parsing git worktree list --porcelain
```go
// Source: https://git-scm.com/docs/git-worktree
import (
    "bufio"
    "os/exec"
    "strings"
)

type WorktreeInfo struct {
    Path   string
    Head   string
    Branch string
}

func ListWorktrees(repoPath string) ([]WorktreeInfo, error) {
    cmd := exec.Command("git", "worktree", "list", "--porcelain")
    cmd.Dir = repoPath

    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var worktrees []WorktreeInfo
    var current WorktreeInfo

    scanner := bufio.NewScanner(strings.NewReader(string(output)))
    for scanner.Scan() {
        line := scanner.Text()

        if line == "" {
            // Empty line separates worktrees
            if current.Path != "" {
                worktrees = append(worktrees, current)
                current = WorktreeInfo{}
            }
            continue
        }

        parts := strings.SplitN(line, " ", 2)
        if len(parts) != 2 {
            continue
        }

        switch parts[0] {
        case "worktree":
            current.Path = parts[1]
        case "HEAD":
            current.Head = parts[1]
        case "branch":
            current.Branch = strings.TrimPrefix(parts[1], "refs/heads/")
        }
    }

    // Add final worktree
    if current.Path != "" {
        worktrees = append(worktrees, current)
    }

    return worktrees, scanner.Err()
}
```

### Dry-Run Merge Conflict Detection
```go
// Source: https://git-scm.com/docs/git-merge-tree
func DetectMergeConflicts(repoPath, baseBranch, taskBranch string) (hasConflicts bool, err error) {
    // git merge-tree --write-tree performs merge without touching working tree
    // Returns tree hash on success, error on conflict
    cmd := exec.Command("git", "merge-tree", "--write-tree", baseBranch, taskBranch)
    cmd.Dir = repoPath

    output, err := cmd.CombinedOutput()
    if err != nil {
        // Non-zero exit = conflicts detected
        return true, nil
    }

    // Clean merge returns tree hash
    return false, nil
}
```

### Signal-Based Graceful Shutdown
```go
// Source: https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/
func RunWithGracefulShutdown(tasks []*Task) error {
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer cancel()

    // Cleanup tracker
    worktrees := make([]WorktreeInfo, 0)
    defer func() {
        for _, wt := range worktrees {
            cleanupWorktree(wt)
        }
    }()

    // Execute tasks
    err := executeTasksInParallel(ctx, tasks, &worktrees)

    if ctx.Err() != nil {
        return fmt.Errorf("shutdown signal received: %w", ctx.Err())
    }

    return err
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| sync.WaitGroup + manual error channels | errgroup.WithContext + SetLimit | Go 1.17 (errgroup.SetLimit added) | Eliminates boilerplate for bounded concurrency and error aggregation |
| git recursive merge strategy | git ort merge strategy | Git 2.33.0 (Aug 2021) | Fewer false conflicts, better rename detection, faster performance |
| Manual git worktree cleanup scripts | git worktree prune + automated tracking | Git 2.17+ (gc.worktreePruneExpire) | Automatic stale worktree cleanup, reduces orphaned branches |
| Unbuffered channels + goroutine per worker | Buffered channels + select | Go 1.0+ (pattern, not feature) | Non-blocking communication without goroutine overhead |
| Custom parallel AI orchestration | clash-sh/clash, Worktrunk (2025-2026) | Released 2025-2026 | Production-ready worktree orchestration for AI agents, conflict detection, TUI monitoring |

**Deprecated/outdated:**
- `git merge -s recursive`: Synonym for `ort` since Git 2.50.0, use `-s ort` explicitly
- go-git worktree API: Incomplete support for prune, lock, repair operations as of 2026
- errgroup without SetLimit: Still valid but recommended to always set limit for resource control

## Open Questions

1. **What merge strategy should be default for AI agent results?**
   - What we know: `ort` is default and handles most cases. `ours`/`theirs` available for policy-based resolution.
   - What's unclear: Whether AI agents should always use `-X ours` (orchestrator wins) or detect conflicts and pause.
   - Recommendation: Make merge strategy configurable per workflow. Default to `ort`, fail on conflict, notify orchestrator. For review tasks, use `-X theirs` (let human edits win).

2. **How many questions will saturate orchestrator Q&A channel?**
   - What we know: Buffered channel size 10 is typical for worker pools. Orchestrator answering is non-blocking to other workers.
   - What's unclear: If answering via LLM, response time may be 2-10 seconds. If all 4 workers ask simultaneously, queue depth matters.
   - Recommendation: Buffer size = 2x concurrency limit (e.g., limit 4 → buffer 8). Monitor channel length, log warnings at 75% capacity.

3. **Should worktrees be persistent or ephemeral?**
   - What we know: Cleanup on success is clean. Preserving on failure aids debugging.
   - What's unclear: Whether to reuse worktrees across runs (faster) or recreate each time (cleaner).
   - Recommendation: Start with ephemeral (create/cleanup each run). Add persistent mode later with `git worktree repair` health checks.

4. **How to handle subprocess agent asking questions back to orchestrator?**
   - What we know: Backend interface has Send() for user messages, but no callback mechanism for agent-initiated questions.
   - What's unclear: Whether to extend Backend interface with QuestionCallback or use separate communication channel.
   - Recommendation: Extend backend.Config with optional QuestionCallback func(question string) (answer string, error). Pass to Backend.New(), backends invoke during Send() when agent asks question. Requires detecting question prompts in agent output (agent-specific parsing).

## Sources

### Primary (HIGH confidence)
- [Git worktree official documentation](https://git-scm.com/docs/git-worktree) - Commands, lifecycle, limitations
- [Git merge strategies official documentation](https://git-scm.com/docs/merge-strategies) - Strategy types, options, behavior
- [errgroup package documentation v0.9.0](https://pkg.go.dev/golang.org/x/sync/errgroup) - SetLimit, TryGo, WithContext behavior
- [Go context package](https://pkg.go.dev/context) - Cancellation patterns
- [Go os/exec package](https://pkg.go.dev/os/exec) - Subprocess execution

### Secondary (MEDIUM confidence)
- [Mastering Git Worktrees with Claude Code](https://medium.com/@dtunai/mastering-git-worktrees-with-claude-code-for-parallel-development-workflow-41dc91e645fe) - Parallel development workflow patterns
- [ccswarm multi-agent orchestration](https://github.com/nwiizo/ccswarm) - Real-world worktree + AI agent implementation
- [How to Use errgroup for Parallel Operations](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view) - Recent errgroup patterns (Jan 2026)
- [Implementing Graceful Shutdown in Go](https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/) - Signal handling patterns
- [clash-sh/clash](https://github.com/clash-sh/clash) - Conflict detection for parallel AI agents with worktrees
- [Worktrunk Git Worktree CLI](https://ascii.co.uk/news/article/news-20260101-05a8cecc/worktrunk-open-sources-git-worktree-cli-for-parallel-ai-agen) - 2026 tool for parallel AI agents

### Tertiary (LOW confidence)
- [Git Worktrees parallel AI development](https://sgryt.com/posts/git-worktree-parallel-ai-development/) - Strategic implementation guide
- [Worker pool patterns](https://gobyexample.com/worker-pools) - Basic Go concurrency

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - errgroup and git worktree are official, documented, proven in production AI agent systems (ccswarm, clash, Worktrunk)
- Architecture: HIGH - Patterns verified with official docs (errgroup, git worktree), real-world implementations (ccswarm, clash), and current 2026 best practices
- Pitfalls: HIGH - Documented in official git docs (branch exclusivity, prune), errgroup docs (SetLimit timing), and community tools exist specifically to solve these (clash conflict detection, git-wt cleanup)

**Research date:** 2026-02-10
**Valid until:** 2026-03-12 (30 days - stable domain, git and Go stdlib change slowly)
