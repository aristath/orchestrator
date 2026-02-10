# Pitfalls Research

**Domain:** Multi-Agent AI Orchestrator (Forking Crush/Charm)
**Researched:** 2026-02-10
**Confidence:** MEDIUM-HIGH

## Critical Pitfalls

### Pitfall 1: Subprocess Output Pipe Deadlocks

**What goes wrong:**
The orchestrator hangs indefinitely when satellite agents (Claude Code CLI, Codex CLI subprocesses) produce output exceeding pipe buffer capacity (typically 64KB). The child process blocks on write() while the parent never reads, or vice versa, creating a deadlock where neither process can proceed.

**Why it happens:**
Go's `exec.Cmd` with `StdoutPipe()` and `StderrPipe()` requires explicit concurrent reading before calling `Wait()`. Developers often call `Wait()` before consuming all pipe data, or read stdout/stderr sequentially instead of concurrently. When one pipe fills while reading from the other, the subprocess blocks writing and never completes.

**How to avoid:**
- Always start goroutines to read from BOTH stdout and stderr BEFORE calling `cmd.Start()`
- Use `io.Copy` in separate goroutines for each stream
- Never call `cmd.Wait()` until all pipe reads complete
- Consider using `cmd.CombinedOutput()` for short-lived commands where interleaving stdout/stderr is acceptable
- Set up buffered channels to stream output to TUI without blocking subprocess

**Warning signs:**
- Agent subprocess shows as running in `ps` but produces no output
- Orchestrator hangs on specific tasks that generate verbose output
- Deadlock occurs inconsistently based on output volume
- Works with small files, hangs with large codebases
- `strace` shows subprocess blocked on write() syscall

**Phase to address:**
Phase 1: Core Subprocess Management - Implement proper concurrent pipe reading before building TUI integration

---

### Pitfall 2: Zombie Process Accumulation

**What goes wrong:**
Failed or killed satellite agents leave zombie processes consuming process table entries. Over extended orchestrator sessions (long development days), zombie accumulation exhausts available PIDs, preventing new agent launches. Zombies don't consume CPU/memory but make the system unable to fork new processes.

**Why it happens:**
Parent process (orchestrator) fails to call `wait()` or `waitpid()` on terminated children. In Go, if `cmd.Wait()` is never called after a process exits (crash, SIGKILL, panic), the child remains as a zombie. Process groups complicate this: killing parent with `os.Process.Kill()` sends SIGKILL only to that PID, not its children.

**How to avoid:**
- Use `syscall.SysProcAttr` with `Setpgid: true` to create process groups
- Send signals to `-pgid` to kill entire process tree
- Register SIGCHLD handler or use `cmd.Wait()` even for killed processes
- Implement process tracking: map[pid]*exec.Cmd with cleanup routines
- Use context with timeout and defer cleanup: `defer cmd.Wait()`
- Consider process reaping goroutine that periodically calls `Wait()` with `WNOHANG`
- For containerized deployments, use Tini or similar init system as PID 1

**Warning signs:**
- `ps aux | grep '<defunct>'` shows increasing zombie count
- "fork: Resource temporarily unavailable" errors
- Agent launches fail after orchestrator runs for hours
- Zombie count increases after agent crashes or manual task cancellation
- Process count grows but resource usage stays flat

**Phase to address:**
Phase 1: Core Subprocess Management - Establish process lifecycle management before parallel execution

---

### Pitfall 3: Fork Drift from Upstream Crush

**What goes wrong:**
After 12-18 months, the forked orchestrator codebase diverges so significantly from upstream Crush that security patches and feature updates become impossible to merge. Critical CVE fixes can't be applied without massive rebasing effort. The fork becomes an isolated, unmaintained branch requiring full-time maintenance.

**Why it happens:**
Aggressive customization without maintaining merge paths. Adding orchestrator logic deeply intertwined with Crush's core structures. Each upstream change touches files modified for orchestration. Delaying rebases causes exponential conflict growth. Configuration-through-customization instead of configuration-through-parameters.

**How to avoid:**
- Use plugin/extension architecture: inject orchestrator behavior via interfaces
- Keep Crush core files unmodified; wrap or extend instead
- Create `orchestrator/` package separate from Crush's core packages
- Use Go interfaces to intercept Crush behavior at boundaries
- Rebase weekly/biweekly (atomic commits make this survivable)
- Maintain configuration layer: environment variables, config files for orchestrator behavior
- Document every deviation from upstream with rationale
- Submit PRs upstream for hooks/interfaces needed for orchestration
- Track upstream releases via GitHub notifications/RSS
- Use `git range-diff` to review incoming changes before rebasing

**Warning signs:**
- Merge conflicts on 50%+ files during rebase attempts
- Upstream changes to core structs break orchestrator logic
- "Too many conflicts, starting from scratch" discussions
- Security advisories can't be applied for months
- Team avoids rebasing, saying "too risky now"
- Crush maintainers make architectural changes incompatible with fork

**Phase to address:**
Phase 0: Fork Strategy Design - Before writing orchestrator code, design extension points
Phase 1: Initial Fork - Implement extension architecture, verify clean rebase process

---

### Pitfall 4: Multi-Agent File Conflict Cascades

**What goes wrong:**
Two satellite agents simultaneously edit the same file. Agent A writes changes, Agent B writes different changes seconds later, silently overwriting A's work. Neither agent detects the conflict. The orchestrator shows both tasks as "successful" but the codebase is corrupted with partial implementations and logic inconsistencies. User doesn't discover the problem until CI fails or they read the code.

**Why it happens:**
No file-level locking mechanism. DAG defines task dependencies but not resource (file) exclusivity. Agents operate independently via subprocesses, unaware of each other's file targets. Race condition: both agents read file, both modify in memory, both write. The orchestrator sees successful exit codes and assumes success.

**How to avoid:**
- Implement file-level lock registry before task execution: `map[filepath]taskID`
- Pre-declare file targets in task metadata: `task.WillModify: []string{"src/foo.go"}`
- DAG scheduler checks lock registry before spawning agent
- Block dependent tasks until file locks release
- Use Git worktrees: each agent gets isolated working tree (same .git)
- Post-task validation: `git diff --check` for conflicts before marking success
- Task-level atomic commits: agent commits its changes, orchestrator resolves conflicts
- Consider "file ownership" strategy: each task claims files, conflicts abort at plan time
- For read-only tasks: allow concurrent access, track with read-write lock semantics

**Warning signs:**
- "This agent undid my changes" user reports
- Git history shows interleaved commits touching same lines
- CI failures with "undefined variable" or incomplete refactors
- Code review reveals half-finished feature from multiple agents
- Merge conflicts when user tries to commit after agents finish
- Tests pass individually but fail when tasks run in parallel

**Phase to address:**
Phase 2: DAG Scheduler - Add resource locking to task scheduling logic before enabling parallel execution
Phase 3: TUI Integration - Show file lock status in split panes

---

### Pitfall 5: Context Window State Drift Across Multi-Turn Conversations

**What goes wrong:**
After 10-15 conversation turns, agents "forget" the original task goal. They start suggesting changes that contradict earlier decisions or repeat already-completed work. The orchestrator's satellite agents lose coherence, making nonsensical changes that technically complete their immediate prompt but violate the overarching project goal.

**Why it happens:**
LLMs have fixed context windows. Each turn adds prompt + response tokens. Eventually, early conversation context (containing the project goal, architecture decisions) gets truncated or poorly compressed. Aggressive summarization loses critical details. Agents work from "recent context" without understanding "why we're doing this." Multi-agent orchestration exacerbates this: each satellite agent has isolated context, unaware of other agents' decisions.

**How to avoid:**
- Implement durable state management separate from LLM context:
  - Project goal (immutable, always included)
  - Architecture decisions log (ADR format)
  - Completed tasks with summaries
  - Active constraints/requirements
- Use semantic compression, not naive truncation: LangChain's "context management for deep agents" pattern
- Every agent prompt includes: `GOAL: [original task]` + `CONTEXT: [relevant ADR]` + `COMPLETED: [related tasks]`
- Detect drift: if agent response mentions completed task, inject reminder
- Short-term tool memory: selectively evict stale tool results from context
- Session state architecture: separate "durable state" from "per-call working context"
- Implement "check against goal" validation: does this change advance the original objective?
- Use hierarchical context: orchestrator maintains full context, satellites get filtered views

**Warning signs:**
- Agent suggests "let's add feature X" when X was added 5 turns ago
- Contradictory implementations: agent A says "use REST", agent B later suggests "let's use GraphQL"
- User has to repeatedly remind agents of project architecture
- Agents ask for information already provided earlier in conversation
- Code changes that seem sensible in isolation but violate project patterns
- Orchestrator task descriptions become vaguer over time

**Phase to address:**
Phase 4: State Management - Before scaling to 10+ parallel agents, implement durable state architecture
Phase 5: Context Optimization - Add semantic compression and drift detection

---

### Pitfall 6: DAG Circular Dependency Detection Failure

**What goes wrong:**
The orchestrator accepts a task graph with circular dependencies (A depends on B, B depends on C, C depends on A). At execution time, the scheduler deadlocks: all three tasks wait for dependencies that never complete. The orchestrator hangs indefinitely showing "waiting for dependencies" with no progress. User force-quits and loses work.

**Why it happens:**
DAG validation runs after user input, not before accepting the graph. Circular dependencies introduced incrementally: user adds "A depends on B", later adds "B depends on C", later adds "C depends on A". Each individual dependency seems valid. No topological sort validation. Dynamic dependencies (added at runtime based on agent decisions) bypass validation.

**How to avoid:**
- Run topological sort (Kahn's algorithm or DFS-based cycle detection) BEFORE accepting task graph
- Validate on every edge addition, not just at execution time
- Detect cycles during task definition: reject dependency if it would create cycle
- Implement graph visualization in TUI showing dependency flow (makes cycles visible)
- For dynamic dependencies (agent-generated): require validation before adding to graph
- Add "max dependency chain depth" limit to prevent accidentally complex graphs
- Provide clear error: "Cannot add dependency: would create cycle A -> B -> C -> A"
- Consider "soft dependencies" (prefer, not require) for complex scenarios
- Serialize task graph to JSON/YAML for inspection and debugging

**Warning signs:**
- Orchestrator hangs with all tasks showing "waiting"
- No tasks transition to "running" state after start
- DAG visualization (if implemented) shows obvious loops
- Graph complexity increases, performance degrades
- User says "I'm not sure why this isn't starting"
- Logs show same task checking dependencies repeatedly

**Phase to address:**
Phase 2: DAG Scheduler - Implement cycle detection before any task execution logic
Phase 3: TUI Integration - Add graph visualization to expose cycles visually

---

### Pitfall 7: Cascading Failure Without Isolation

**What goes wrong:**
One satellite agent task fails (API timeout, LLM error, out of memory). The orchestrator immediately fails all downstream tasks without attempting them. Work that could have succeeded independently is aborted. Worse: a transient failure (temporary network issue) causes complete workflow failure requiring full restart and wasting 10+ minutes of LLM processing.

**Why it happens:**
Naive dependency handling: "if any dependency failed, fail this task." No distinction between hard failures (code error) and soft failures (retry-able error). No isolation between dependency chains: task F depends on E, E depends on D, D fails. Tasks G, H, I (unrelated to D/E/F) also abort because the orchestrator enters "failure mode." All-or-nothing execution model.

**How to avoid:**
- Classify failures: `HARD_FAIL` (code error), `SOFT_FAIL` (retry-able), `SKIP` (optional task)
- Implement retry logic with exponential backoff for soft failures
- Isolate dependency chains: failure in chain A doesn't affect chain B
- Support partial success: mark failed task as skipped, continue with independent tasks
- Add "continue on error" flag for optional tasks
- Checkpoint completed tasks: restarting resumes from last success, not from scratch
- Provide user choice on failure: "Retry / Skip / Abort all / Continue others"
- Implement failure budgets: allow N failures before aborting entire workflow
- Use idempotent tasks: re-running doesn't break already-completed work
- Log failure context: why it failed, was it transient, should we retry

**Warning signs:**
- Entire workflow fails due to one flaky API call
- User manually re-runs, succeeds on second attempt (transient failure)
- "All or nothing" behavior: even unrelated tasks abort
- No way to resume partially completed workflows
- Orchestrator exits immediately on first error
- Failed task percentage correlates with unrelated task failures

**Phase to address:**
Phase 2: DAG Scheduler - Implement failure classification and isolation before parallel execution
Phase 5: Resilience - Add checkpointing and retry logic for production readiness

---

### Pitfall 8: TUI State Desync with Actual Agent Status

**What goes wrong:**
The TUI shows "Agent A: Running" but Agent A crashed 30 seconds ago. Or TUI shows task completed but the file changes aren't present. User makes decisions based on stale UI state. The orchestrator's internal state and displayed state diverge. User restarts the orchestrator thinking it's hung when it's actually working.

**Why it happens:**
TUI render loop and agent subprocess management run independently. No synchronization mechanism. Agent status checked once at task start, not continuously monitored. Process exit not detected until `Wait()` is called (maybe never called). Bubble Tea message passing loses messages under load. Race condition: agent updates state, TUI renders from old state, update arrives after render.

**How to avoid:**
- Implement process monitoring goroutine per agent: polls `os.Process` for exit
- Use channels to communicate state changes: agent goroutine -> orchestrator -> TUI
- Bubble Tea Cmd pattern: state changes produce messages that trigger re-renders
- Heartbeat mechanism: agent subprocess sends periodic "alive" signals
- Status validation: periodically verify subprocess PID still exists
- Atomic state updates: use mutex or sync.Map for shared state accessed by goroutines and TUI
- Decouple state from view: TUI always renders from authoritative state store
- Add "last updated" timestamp in UI to indicate state freshness
- Log state transitions: when orchestrator updates agent status, log with timestamp
- Handle race conditions: if TUI renders "running" but process exited, next tick corrects it

**Warning signs:**
- User reports "UI shows running but nothing's happening"
- Process list shows no agent subprocess but TUI shows active
- File changes appear but TUI never showed completion
- TUI flickers between states rapidly (competing updates)
- Agent status lags behind actual progress by seconds/minutes
- Force-killing orchestrator leaves agents running (UI lost track)

**Phase to address:**
Phase 3: TUI Integration - Implement proper state synchronization from the start, not as afterthought
Phase 4: Monitoring - Add health checks and heartbeat mechanisms

---

### Pitfall 9: Signal Propagation Failure to Subprocess Trees

**What goes wrong:**
User hits Ctrl+C to stop the orchestrator. Orchestrator exits, but all satellite agents (Claude Code CLI, Codex CLI) continue running in background. User's terminal returns, but agents keep modifying files. Multiple orchestrator sessions accumulate orphaned agents consuming API credits and CPU. Kill requires manual `pkill` or system restart.

**Why it happens:**
Go's `os.Process.Kill()` sends SIGKILL only to the direct child PID, not its children. AI agent CLIs often spawn subprocesses (Node.js, Python scripts). Signal doesn't propagate. Default signal handling: SIGINT caught by orchestrator but not forwarded to children. Subprocess started without process group setup.

**How to avoid:**
- Use `syscall.SysProcAttr` with `Setpgid: true` when creating `exec.Cmd`
- Assign new process group ID: child and all descendants share PGID
- On signal, kill process group: `syscall.Kill(-pgid, syscall.SIGTERM)`
- Register signal handler: `signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)`
- Graceful shutdown sequence:
  1. Send SIGTERM to all agent process groups
  2. Wait 5 seconds for clean exit
  3. Send SIGKILL to stragglers
  4. Clean up temp files, sockets
- Handle context cancellation: `context.WithCancel()` propagates cancellation to agent goroutines
- Track all spawned PIDs: maintain registry to kill on exit even if PGID fails
- Test signal handling: simulate SIGINT in test suite, verify all processes exit

**Warning signs:**
- `ps aux` shows multiple orphaned Claude Code/Codex processes
- API usage continues after orchestrator exits
- File modifications appear after user thinks orchestrator stopped
- Need to manually kill processes after each session
- System accumulates background processes over time
- "Address already in use" errors (orphaned process holding port)

**Phase to address:**
Phase 1: Core Subprocess Management - Implement process group and signal handling before any multi-agent logic

---

### Pitfall 10: Satellite Agent Failure Mid-Task Without Recovery

**What goes wrong:**
Agent successfully completes 60% of a complex refactoring task (10 files modified), then crashes or is killed. The orchestrator marks task as failed and discards all work. User manually inspects and finds 6 files correctly modified, 4 files untouched. Manual recovery required: figure out what's done, what's not, complete manually. Wasted LLM processing and user time.

**Why it happens:**
No intermediate state persistence. Agent work is all-or-nothing: success commits everything, failure commits nothing. Crash loses in-memory state. No checkpointing during long-running tasks. Orchestrator can't resume mid-task: only knows "started" or "completed." No structured work log from satellite agent. Impossible to determine what was accomplished before failure.

**How to avoid:**
- Require satellite agents to checkpoint progress: commit after each file or logical unit
- Task decomposition: break "refactor 10 files" into 10 tasks of "refactor 1 file"
- Use structured work log: agent writes `task-123-progress.json` with completed steps
- Orchestrator reads progress log: can resume from last checkpoint
- Git-based checkpointing: agent commits after each successful file edit with marker message
- On failure, offer resume: "Agent failed at step 7/10. Retry from step 7?"
- Idempotent operations: rerunning steps 1-6 doesn't break if already done
- Implement agent protocol: `CHECKPOINT`, `RESUME_FROM`, `GET_PROGRESS` commands
- State machine for tasks: `NOT_STARTED -> IN_PROGRESS -> CHECKPOINTED -> COMPLETED -> FAILED`
- Graceful degradation: partial success marked as "incomplete" not "failed"

**Warning signs:**
- Long-running tasks frequently restart from scratch after errors
- User manually completes partially-done work from failed agents
- Git history shows no commits from failed tasks (all work lost)
- Orchestrator can't answer "what did the agent accomplish before crashing?"
- Retry always starts over, ignoring completed parts
- Agent crashes after 10 minutes, user repeats 10 minutes of API calls

**Phase to address:**
Phase 4: State Management - Add checkpointing and progress tracking before production use
Phase 5: Resilience - Implement resume-from-checkpoint logic

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Sequential pipe reading (stdout then stderr) | Simpler code, fewer goroutines | Deadlocks on buffer overflow (Pitfall 1) | Never - use concurrent reading |
| Skipping `cmd.Wait()` for background processes | Faster task completion appearance | Zombie accumulation (Pitfall 2) | Never - always clean up |
| Direct modification of Crush core files | Faster feature implementation | Fork drift, unmergeable upstream (Pitfall 3) | Never - use extension architecture |
| No file-level locking in DAG scheduler | Simpler scheduler logic | Race conditions, data corruption (Pitfall 4) | MVP only, must add before multi-agent |
| Naive context concatenation for agents | Easy to implement | Context window overflow, state drift (Pitfall 5) | Single-agent only, not multi-agent |
| Skip topological sort validation | Faster graph construction | Runtime deadlocks from cycles (Pitfall 6) | Never - validation is O(n), deadlock is infinite |
| Abort all tasks on first failure | Simple failure handling | Wasted work, poor UX (Pitfall 7) | Early prototype only |
| Poll process status every second | Real-time UI updates | CPU overhead, doesn't catch rapid state changes | Use event-driven updates instead |
| No SIGCHLD handler or process groups | Works on happy path | Signal propagation failure (Pitfall 9) | Never - orphaned processes are unacceptable |
| All-or-nothing task completion | No state management needed | Lost work on failure (Pitfall 10) | Short tasks (<1min) only |

## Integration Gotchas

Common mistakes when connecting to external services and CLIs.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Claude Code CLI | Assuming stable JSON output format | Parse with schema validation, handle version differences |
| Codex CLI | Blocking on subprocess.communicate() | Use MCP stdio transport with async message handling |
| Git operations from agents | Agent commits without checking working tree state | Pre-commit: check for conflicts, staged changes, detached HEAD |
| LLM API calls | No timeout, retry on all errors equally | Timeout per call, classify errors (rate limit vs auth vs transient) |
| File system operations | Assume agent has write permission | Check permissions before spawning agent, fail fast with clear error |
| Process cleanup | Only kill parent PID | Use process groups (Pitfall 9) to kill entire tree |
| Pipe communication | Sequential read of stdout, stderr | Concurrent goroutines for both streams (Pitfall 1) |
| TUI rendering | Block main thread on agent operations | Use Bubble Tea async commands, never block render loop (Pitfall 8) |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Unbounded context accumulation | Increasing latency per turn, eventual token limit errors | Implement semantic compression and pruning (Pitfall 5) | 15+ conversation turns |
| No stdout/stderr buffering strategy | Deadlocks on verbose output | Use buffered channels or io.Pipe with goroutines (Pitfall 1) | >64KB output per agent |
| Synchronous task execution | Only one agent active at a time | DAG-based parallel execution with resource locking (Pitfall 4) | 10+ tasks in workflow |
| Full TUI re-render on every state change | UI lag, high CPU usage | Differential rendering, only update changed panes | 5+ active agents |
| No task result caching | Repeat identical API calls | Cache LLM responses by (task_type, file_hash, prompt_hash) | Workflows with repeated patterns |
| Naive process table polling | `ps aux` parsing, high syscall overhead | Use process monitoring via channels and `os.Process` APIs | 20+ concurrent agents |
| No garbage collection of completed task state | Memory grows indefinitely | Archive old task results to disk, keep recent N in memory | Multi-hour sessions |
| Blocking on git operations | UI freezes during commits | Run git commands in goroutines, show progress in TUI | Large repos (1000+ files) |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Running satellite agents with full filesystem access | Agent compromise leads to system compromise | Implement .crushignore, restrict to project directory, sandbox via containers |
| No validation of agent-generated code before execution | Agent hallucinates malicious command, runs it | Require approval for shell commands, file system writes outside project dir |
| Passing raw user input as LLM prompt | Prompt injection, jailbreaking | Sanitize and structure prompts, use system/user message separation |
| Logging full LLM conversations with secrets | API keys, credentials in logs | Redact sensitive patterns, use structured logging with secret detection |
| Allowing agents to modify .git directory | Corrupt repository, lose history | Whitelist allowed git operations, forbid direct .git/ writes |
| No rate limiting on agent LLM calls | API bill explosion from runaway agent | Per-agent, per-workflow token budgets with hard limits |
| Trusting agent exit codes as success signal | Agent exits 0 but left corrupted state | Validate task postconditions: files exist, syntax valid, tests pass |
| Running orchestrator as root | Privilege escalation via agent exploit | Drop privileges, use unprivileged user, container security contexts |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No visibility into agent's current action | "Is it frozen or working?" anxiety | Stream agent's thinking/actions to TUI in real-time |
| Can't cancel long-running task | Forced to kill orchestrator, lose all work | Implement graceful cancellation (Pitfall 9) + save partial work (Pitfall 10) |
| Cryptic error messages | User doesn't know what went wrong or how to fix | Contextual errors: "Agent failed because X. Try: Y. Logs: Z" |
| No progress indication for multi-step tasks | Appears hung on complex refactors | Show step N/M, current file, time elapsed |
| TUI overload with all agent outputs | Information overload, can't find relevant info | Collapsible panes, log levels, filtering by agent/task |
| Silent failures | Task marked complete but file unchanged | Always show diff/summary of what changed |
| No way to review before committing | Agent makes unexpected changes, user forced to accept | Offer review mode: show changes, accept/reject/modify |
| Unclear task dependencies | User confused why task not starting | Visualize DAG with blocked/waiting/running states (Pitfall 6) |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Subprocess management:** Often missing concurrent pipe reading — verify goroutines for stdout AND stderr before `Wait()` (Pitfall 1)
- [ ] **Process lifecycle:** Often missing `Wait()` calls for killed processes — verify zombie process cleanup with `ps aux | grep defunct` (Pitfall 2)
- [ ] **Signal handling:** Often missing process group setup — verify `Setpgid: true` and SIGTERM propagation test (Pitfall 9)
- [ ] **DAG scheduler:** Often missing cycle detection — verify topological sort runs before execution (Pitfall 6)
- [ ] **File locking:** Often missing lock registry — verify concurrent agent test with same-file edits (Pitfall 4)
- [ ] **State persistence:** Often missing checkpointing — verify agent crash recovery doesn't lose partial work (Pitfall 10)
- [ ] **Context management:** Often missing compression strategy — verify 20+ turn conversation doesn't drift (Pitfall 5)
- [ ] **Error handling:** Often missing failure classification — verify transient error retries, not aborts (Pitfall 7)
- [ ] **TUI synchronization:** Often missing state validation — verify UI shows actual process status, not stale (Pitfall 8)
- [ ] **Fork maintenance:** Often missing upstream sync process — verify clean rebase possible after changes (Pitfall 3)

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Subprocess deadlock (1) | LOW | Send SIGKILL to hung subprocess, restart task. Fix: add goroutines for pipe reading. |
| Zombie accumulation (2) | LOW | `pkill -9 <defunct>`, restart orchestrator. Fix: add `defer cmd.Wait()` everywhere. |
| Fork drift (3) | HIGH | Months of work. Either: freeze fork, maintain separately OR start over with extension architecture. |
| File conflicts (4) | MEDIUM | Manual git merge, resolve conflicts. Check both agent outputs for correctness. Fix: add file locking. |
| Context drift (5) | MEDIUM | Start new conversation with fresh context, copy over key decisions. Fix: implement durable state. |
| DAG deadlock (6) | LOW | Ctrl+C, fix task dependencies. Use `dot` to visualize graph, find cycle. Fix: add topological sort. |
| Cascading failure (7) | MEDIUM | Restart from checkpoint if available, else restart workflow. Fix: add failure isolation. |
| TUI desync (8) | LOW | Restart orchestrator. Fix: add process monitoring and atomic state updates. |
| Orphaned agents (9) | LOW | `pkill -f "claude-code\|codex-cli"`, check API usage. Fix: add process groups and signal handling. |
| Lost partial work (10) | MEDIUM | Manual inspection of git diff/logs to salvage completed parts. Fix: add checkpointing. |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Subprocess deadlock (1) | Phase 1: Core Subprocess Management | Test with agent generating 1MB output, verify no hang |
| Zombie accumulation (2) | Phase 1: Core Subprocess Management | Run 100 agent tasks, verify `ps` shows 0 zombies |
| Fork drift (3) | Phase 0: Fork Strategy Design, Phase 1 | Successfully rebase onto Crush main after adding orchestrator |
| File conflicts (4) | Phase 2: DAG Scheduler | Launch 2 agents editing same file, verify conflict detection |
| Context drift (5) | Phase 4: State Management | 30-turn conversation, verify agent remembers turn-1 goal |
| DAG deadlock (6) | Phase 2: DAG Scheduler | Submit circular dependency graph, verify rejection |
| Cascading failure (7) | Phase 2: DAG Scheduler, Phase 5: Resilience | Fail task A, verify independent task B continues |
| TUI desync (8) | Phase 3: TUI Integration | Kill agent subprocess, verify UI updates within 1 second |
| Orphaned agents (9) | Phase 1: Core Subprocess Management | Ctrl+C orchestrator, verify all agents exit cleanly |
| Lost partial work (10) | Phase 4: State Management, Phase 5: Resilience | Crash agent at 50% task completion, verify partial work saved |

## Sources

**Multi-Agent AI Orchestration:**
- [Deloitte: AI Agent Orchestration](https://www.deloitte.com/us/en/insights/industry/technology/technology-media-and-telecom-predictions/2026/ai-agent-orchestration.html)
- [Microsoft Learn: AI Agent Design Patterns](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)
- [UiPath: Challenges Deploying AI Agents](https://www.uipath.com/blog/ai/common-challenges-deploying-ai-agents-and-solutions-why-orchestration)
- [Gartner: 40% Agentic AI Projects Fail](https://byteiota.com/gartner-40-agentic-ai-projects-fail-heres-why/)
- [CloudGeometry: Multi-Agent Systems Architecture](https://www.cloudgeometry.com/blog/from-solo-act-to-orchestra-why-multi-agent-systems-demand-real-architecture)

**Subprocess Management & Zombie Processes:**
- [Stormkit: Hunting Zombie Processes in Go and Docker](https://www.stormkit.io/blog/hunting-zombie-processes-in-go-and-docker)
- [Methods to Avoid Zombie Processes](https://duyanghao.github.io/ways_avoid_zombie_process/)
- [GeeksforGeeks: Zombie Processes Prevention](https://www.geeksforgeeks.org/zombie-processes-prevention/)

**Fork Maintenance:**
- [Stop Forking Around: Fork Drift Dangers](https://preset.io/blog/stop-forking-around-the-hidden-dangers-of-fork-drift-in-open-source-adoption/)
- [How to Fork: Best Practices](https://joaquimrocha.com/2024/09/22/how-to-fork/)
- [rOpenSci: Forks and Upstream Relationship](https://ropensci.org/blog/2025/02/20/forks-upstream-relationship/)

**DAG Scheduling:**
- [Apache Airflow DAG Crashes Due to Circular Dependencies](https://www.shoreline.io/runbooks/airflow/apache-airflow-dag-crashes-due-to-circular-or-complex-task-dependencies)
- [Managing Dependencies in Apache Airflow DAGs](https://www.cloudthat.com/resources/blog/managing-dependencies-in-apache-airflow-dags)

**Multi-Agent File Conflicts:**
- [Claude Code Multiple Agent Systems Guide 2026](https://www.eesel.ai/blog/claude-code-multiple-agent-systems-complete-2026-guide)
- [Using Git Worktrees for Parallel AI Development](https://stevekinney.com/courses/ai-development/git-worktrees)
- [Multi-Agent Coordination MCP Server](https://github.com/AndrewDavidRivers/multi-agent-coordination-mcp)
- [Mission Control for AI Agents](https://skywork.ai/blog/agent/mission-control-for-ai-agents-managing-multiple-agents-in-github/)

**Context Window Management:**
- [Context Window Management Strategies 2026](https://www.getmaxim.ai/articles/context-window-management-strategies-for-long-context-ai-agents-and-chatbots/)
- [Context Window Overflow Fix 2026](https://redis.io/blog/context-window-overflow/)
- [JetBrains: Efficient Context Management](https://blog.jetbrains.com/research/2025/12/efficient-context-management/)
- [LangChain: Context Management for Deep Agents](https://blog.langchain.com/context-management-for-deepagents/)

**Agent Failure Recovery:**
- [Error Handling in Agentic Systems](https://agentsarcade.com/blog/error-handling-agentic-systems-retries-rollbacks-graceful-failure)
- [Why Multi-Agent LLM Systems Fail](https://www.augmentcode.com/guides/why-multi-agent-llm-systems-fail-and-how-to-fix-them)
- [Agent Error Handling & Recovery](https://apxml.com/courses/langchain-production-llm/chapter-2-sophisticated-agents-tools/agent-error-handling)

**Go Subprocess Pipe Deadlocks:**
- [Go os/exec Package Documentation](https://pkg.go.dev/os/exec)
- [Running External Programs in Go: The Right Way](https://medium.com/@caring_smitten_gerbil_914/running-external-programs-in-go-the-right-way-38b11d272cd1)
- [Reading os/exec.Cmd Output Without Race Conditions](https://hackmysql.com/rand/reading-os-exec-cmd-output-without-race-conditions/)
- [DoltHub: Useful Patterns for Go's os/exec](https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/)
- [GitHub Issue: io/ioutil hangs with too big output](https://github.com/golang/go/issues/16787)

**Charm/Crush:**
- [Charm Crush GitHub Repository](https://github.com/charmbracelet/crush)
- [Crush CLI: Next-Generation AI Coding Agent](https://atalupadhyay.wordpress.com/2025/08/12/crush-cli-the-next-generation-ai-coding-agent/)

**Codex CLI:**
- [Building Consistent Workflows with Codex CLI & Agents SDK](https://cookbook.openai.com/examples/codex/codex_mcp_agents_sdk/building_consistent_workflows_codex_cli_agents_sdk)
- [Codex CLI Multi-Agent Orchestration Production Guide](https://alirezarezvani.medium.com/openai-codex-cli-from-broken-install-to-multi-agent-orchestration-production-guide-07d8b7d513ef)

**TUI/Bubble Tea:**
- [Charm Bubble Tea GitHub](https://github.com/charmbracelet/bubbletea)
- [Exploring Bubble Tea TUI Framework](https://www.oreateai.com/blog/exploring-bubble-tea-a-delightful-journey-into-gos-tui-framework/0f857b4aed9849e12d7280c76b572c57)

---
*Pitfalls research for: Multi-Agent AI Orchestrator*
*Researched: 2026-02-10*
