# Phase 2: Agent Definitions and DAG Scheduler - Research

**Researched:** 2026-02-10
**Domain:** DAG-based task scheduling, configuration management, multi-agent orchestration
**Confidence:** HIGH

## Summary

Phase 2 builds the core orchestration engine: agent role definitions, configuration hierarchy, and DAG-based task scheduling with dependency resolution. The Go ecosystem provides mature libraries for all critical components: topological sorting with cycle detection (gammazero/toposort), configuration management with merge capabilities (gookit/config), and fine-grained file-level locking (mapmutex pattern).

The architecture requires three distinct layers: (1) configuration system with global/project merge, (2) DAG scheduler with Kahn's algorithm for topological sort and cycle detection, and (3) resource locking layer to prevent concurrent file writes. The existing Phase 1 backend abstraction (Backend interface with Send/Receive) provides the foundation for agent execution.

**Primary recommendation:** Use proven libraries rather than implementing core algorithms from scratch. DAG cycle detection is algorithmically complex and error-prone — use gammazero/toposort with Kahn's algorithm. For configuration, gookit/config provides battle-tested multi-file merging with clear override semantics. Implement file-level resource locking using the keyed mutex pattern (inspired by mapmutex) rather than global locks.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| [gammazero/toposort](https://pkg.go.dev/github.com/gammazero/toposort) | v0.1.1 | DAG topological sort with cycle detection | Uses Kahn's algorithm, battle-tested, automatic cycle detection with error reporting |
| [gookit/config/v2](https://pkg.go.dev/github.com/gookit/config/v2) | v2.2.7+ | Multi-file config loading with merge | Supports JSON/YAML, automatic key-based merging, clear override semantics |
| stdlib sync package | Go 1.25+ | Mutex, RWMutex for resource locking | Standard library, zero dependencies, proven concurrency primitives |
| stdlib encoding/json | Go 1.25+ | JSON marshaling/unmarshaling | Standard library, sufficient for config parsing |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| [kirsle/configdir](https://pkg.go.dev/github.com/kirsle/configdir) | Latest | Cross-platform config directory paths | Get ~/.orchestrator path on Linux/macOS/Windows following OS conventions |
| stdlib context | Go 1.25+ | Timeout, cancellation propagation | Task timeout enforcement, graceful shutdown of agent subprocesses |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gammazero/toposort | Hand-rolled DFS topological sort | Toposort library handles edge cases (isolated nodes, cycle reporting) that custom implementations miss |
| gookit/config | spf13/viper | Viper doesn't support true multi-file merging (single config file per instance), but has larger ecosystem |
| File-level mutex map | sync.Map with global lock | Global lock kills parallelism; per-file locks allow concurrent writes to different files |

**Installation:**
```bash
go get github.com/gammazero/toposort@v0.1.1
go get github.com/gookit/config/v2@latest
go get github.com/kirsle/configdir@latest
```

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── config/          # Configuration loading and merging
│   ├── types.go     # AgentConfig, ProviderConfig, WorkflowConfig structs
│   ├── loader.go    # Load global + project configs, merge
│   └── defaults.go  # Built-in default agent definitions
├── scheduler/       # DAG scheduler
│   ├── dag.go       # DAG construction, cycle detection
│   ├── task.go      # Task node with dependencies, status
│   ├── executor.go  # Task execution with resource locking
│   └── locks.go     # File-level resource lock manager (keyed mutex)
└── backend/         # Already exists from Phase 1
    ├── types.go     # Backend interface
    └── ...
```

### Pattern 1: Configuration Hierarchy with Override Merge

**What:** Load global config (~/.orchestrator/config.json), then project config (.orchestrator/config.json), merge with project values overriding global.

**When to use:** All configuration loading. Allows users to define defaults globally, override per-project.

**Example:**
```go
// Source: gookit/config docs + custom integration
import "github.com/gookit/config/v2"

func LoadConfig() (*OrchestratorConfig, error) {
    c := config.New("orchestrator")

    // Load global config first
    globalPath := filepath.Join(configdir.LocalConfig("orchestrator"), "config.json")
    if err := c.LoadExists(globalPath); err != nil {
        return nil, err
    }

    // Load project config (overrides global)
    projectPath := ".orchestrator/config.json"
    if err := c.LoadExists(projectPath); err != nil {
        return nil, err
    }

    // Unmarshal merged config
    var cfg OrchestratorConfig
    if err := c.Decode(&cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

### Pattern 2: DAG Construction with Kahn's Algorithm

**What:** Build task graph with explicit dependencies, validate with topological sort, reject cycles immediately.

**When to use:** Plan decomposition phase. Before ANY execution, validate the plan is schedulable.

**Example:**
```go
// Source: gammazero/toposort + custom wrapper
import "github.com/gammazero/toposort"

type TaskDAG struct {
    tasks map[string]*Task
    edges []toposort.Edge
}

func (dag *TaskDAG) AddTask(id string, task *Task, dependsOn []string) {
    dag.tasks[id] = task
    for _, depID := range dependsOn {
        // Edge (u, v) means u must come before v
        dag.edges = append(dag.edges, toposort.Edge{depID, id})
    }
}

func (dag *TaskDAG) Validate() ([]string, error) {
    // Topologically sort — returns error if cycle detected
    sorted, err := toposort.Toposort(dag.edges)
    if err != nil {
        return nil, fmt.Errorf("DAG contains cycle: %w", err)
    }

    // Convert []interface{} to []string (task IDs in dependency order)
    order := make([]string, len(sorted))
    for i, id := range sorted {
        order[i] = id.(string)
    }
    return order, nil
}
```

### Pattern 3: Keyed Mutex for File-Level Resource Locking

**What:** Per-file mutex map allowing concurrent writes to different files, blocking only same-file writes.

**When to use:** Before scheduling any write task, acquire lock for target file; release after completion.

**Example:**
```go
// Source: Inspired by github.com/EagleChen/mapmutex pattern
type ResourceLockManager struct {
    mu     sync.Mutex
    locks  map[string]*sync.Mutex
}

func NewResourceLockManager() *ResourceLockManager {
    return &ResourceLockManager{
        locks: make(map[string]*sync.Mutex),
    }
}

func (rlm *ResourceLockManager) Lock(filepath string) {
    rlm.mu.Lock()
    if _, exists := rlm.locks[filepath]; !exists {
        rlm.locks[filepath] = &sync.Mutex{}
    }
    fileLock := rlm.locks[filepath]
    rlm.mu.Unlock()

    fileLock.Lock() // Block until file is unlocked
}

func (rlm *ResourceLockManager) Unlock(filepath string) {
    rlm.mu.Lock()
    fileLock := rlm.locks[filepath]
    rlm.mu.Unlock()

    if fileLock != nil {
        fileLock.Unlock()
    }
}
```

### Pattern 4: Context-Based Task Execution with Timeout

**What:** Use context.WithTimeout to enforce task deadlines, propagate cancellation to agent subprocesses.

**When to use:** Every task execution. Prevents hung agents from blocking scheduler indefinitely.

**Example:**
```go
// Source: Go stdlib context package best practices
func (exec *Executor) ExecuteTask(task *Task) error {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), task.Timeout)
    defer cancel()

    // Acquire resource locks
    for _, file := range task.WritesFiles {
        exec.lockMgr.Lock(file)
        defer exec.lockMgr.Unlock(file)
    }

    // Execute via backend (Phase 1 interface)
    backend, err := exec.getBackend(task.AgentRole)
    if err != nil {
        return err
    }

    resp, err := backend.Send(ctx, backend.Message{
        Content: task.Prompt,
        Role:    "user",
    })

    if ctx.Err() == context.DeadlineExceeded {
        return fmt.Errorf("task timed out after %v", task.Timeout)
    }

    return err
}
```

### Anti-Patterns to Avoid

- **Don't use DFS-based topological sort without cycle detection** — Easy to miss cycle detection logic, harder to debug than Kahn's algorithm with in-degree tracking
- **Don't merge configs manually** — gookit/config handles nested map merging correctly; manual merging misses edge cases
- **Don't use sync.Map for file locks** — sync.Map is for write-once/read-many or disjoint keys; file locking needs traditional mutex map
- **Don't create new Backend per task** — Reuse Backend instances per agent role; subprocess startup overhead is expensive

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Topological sort + cycle detection | DFS with visited tracking | gammazero/toposort (Kahn's algorithm) | Cycle detection in DFS requires careful coloring (white/gray/black); Kahn's algorithm detects cycles naturally via in-degree tracking |
| Config file merging | Recursive map merge | gookit/config LoadExists + Decode | Nested map merging has edge cases (slice append vs replace, nil vs empty); library handles it |
| Cross-platform config paths | String concatenation with os.PathSeparator | kirsle/configdir | Windows vs Unix conventions differ (~/.config vs %APPDATA%); library handles OS-specific paths |
| Task timeout enforcement | time.After() + select | context.WithTimeout | Context propagates cancellation to child goroutines; time.After leaks goroutines |

**Key insight:** DAG scheduling and configuration merging are deceptively complex. A naive cycle detection implementation misses edge cases (disconnected components, self-loops). Config merging fails on nested structures without proper recursion. Use libraries — they've hit the edge cases already.

## Common Pitfalls

### Pitfall 1: Not Checking for Disconnected DAG Components

**What goes wrong:** Topological sort only orders connected nodes; tasks with no dependencies AND no dependents may be lost.

**Why it happens:** Assuming all tasks are transitively connected through dependencies.

**How to avoid:** When adding tasks to DAG, track all task IDs. After topological sort, verify sorted order contains ALL task IDs. For isolated tasks (no edges), explicitly add them to the edges slice with nil dependency: `Edge{nil, taskID}` (see toposort docs).

**Warning signs:** Task count before sort ≠ task count after sort; tasks mysteriously never execute.

### Pitfall 2: Deadlock from Incorrect Lock Ordering

**What goes wrong:** Task A locks file1 then file2; Task B locks file2 then file1 → deadlock.

**Why it happens:** Multi-file locks acquired in different order by different tasks.

**How to avoid:** Sort file paths lexicographically before acquiring locks. Always lock in consistent order.

**Warning signs:** Scheduler hangs indefinitely; no tasks complete; goroutine profiles show blocked Lock() calls.

### Pitfall 3: Merging Configs Without Clearing Previous Data

**What goes wrong:** Loading global config, then project config, but old global values persist when they should be overridden.

**Why it happens:** gookit/config merges by default; if project config omits a key, global value remains.

**How to avoid:** This is CORRECT behavior — project config should only override what it explicitly sets. If you need full replacement, use `SetData()` instead of `LoadExists()`.

**Warning signs:** Expected project overrides don't take effect; old global values appear in merged config.

### Pitfall 4: Race Condition in Keyed Mutex Map

**What goes wrong:** Two goroutines call Lock(filepath) simultaneously, both create new mutexes for same file, both proceed into critical section.

**Why it happens:** Check-then-act race: check if mutex exists, then create it if not — not atomic.

**How to avoid:** Guard the map itself with a mutex (see Pattern 3). Lock the map, get/create file mutex, unlock the map, THEN lock the file mutex.

**Warning signs:** Concurrent writes to same file despite locking; data corruption; test failures under -race flag.

### Pitfall 5: Not Propagating Context Cancellation to Backend

**What goes wrong:** Task timeout expires, but agent subprocess keeps running, consuming resources.

**Why it happens:** Backend.Send() receives context but doesn't pass it to exec.CommandContext.

**How to avoid:** Phase 1 backend adapters already use context in executeCommand (see internal/backend/process.go). Ensure Backend.Send() passes context down to executeCommand.

**Warning signs:** Hung agent processes after timeout; increasing process count; resource exhaustion.

### Pitfall 6: Circular Dependency in Workflow Config

**What goes wrong:** Workflow says "after code, run review; after review, run code" → infinite loop.

**Why it happens:** Workflow config creates implicit dependencies that form cycles.

**How to avoid:** When spawning workflow follow-up tasks, validate the ENTIRE graph (original + workflow tasks) with topological sort. Reject workflow configs that create cycles.

**Warning signs:** DAG validation fails only after workflow tasks added; error message mentions review/test tasks.

## Code Examples

Verified patterns from official sources:

### Loading JSON Config with Defaults

```go
// Source: gookit/config v2 documentation
import "github.com/gookit/config/v2"

type AgentConfig struct {
    Role         string `json:"role"`
    Provider     string `json:"provider"`
    Model        string `json:"model"`
    SystemPrompt string `json:"system_prompt"`
    Tools        []string `json:"tools"`
}

func LoadAgentConfigs() (map[string]AgentConfig, error) {
    c := config.New("agents")

    // Load with automatic merging
    if err := c.LoadExists(
        "~/.orchestrator/config.json",
        ".orchestrator/config.json",
    ); err != nil {
        return nil, err
    }

    var agents map[string]AgentConfig
    if err := c.BindStruct("agents", &agents); err != nil {
        return nil, err
    }

    return agents, nil
}
```

### Dependency Resolution with Task Completion

```go
// Source: Custom implementation using topological sort concepts
type Scheduler struct {
    dag           *TaskDAG
    pending       map[string]*Task  // Not yet eligible
    eligible      []*Task           // No unresolved deps
    running       map[string]*Task
    completed     map[string]bool
    dependencies  map[string][]string // taskID -> depends on taskIDs
}

func (s *Scheduler) MarkCompleted(taskID string) {
    s.completed[taskID] = true
    delete(s.running, taskID)

    // Check if any pending tasks are now eligible
    for pendingID, task := range s.pending {
        if s.allDependenciesCompleted(pendingID) {
            delete(s.pending, pendingID)
            s.eligible = append(s.eligible, task)
        }
    }
}

func (s *Scheduler) allDependenciesCompleted(taskID string) bool {
    deps := s.dependencies[taskID]
    for _, depID := range deps {
        if !s.completed[depID] {
            return false
        }
    }
    return true
}
```

### Table-Driven Tests for DAG Validation

```go
// Source: Go wiki TableDrivenTests + best practices
func TestDAGValidation(t *testing.T) {
    tests := []struct {
        name      string
        tasks     map[string][]string // taskID -> depends on
        wantErr   bool
        errContains string
    }{
        {
            name: "valid linear dependency",
            tasks: map[string][]string{
                "task1": {},
                "task2": {"task1"},
                "task3": {"task2"},
            },
            wantErr: false,
        },
        {
            name: "valid parallel tasks",
            tasks: map[string][]string{
                "task1": {},
                "task2": {},
                "task3": {"task1", "task2"},
            },
            wantErr: false,
        },
        {
            name: "cycle detection - direct",
            tasks: map[string][]string{
                "task1": {"task2"},
                "task2": {"task1"},
            },
            wantErr: true,
            errContains: "cycle",
        },
        {
            name: "cycle detection - transitive",
            tasks: map[string][]string{
                "task1": {"task2"},
                "task2": {"task3"},
                "task3": {"task1"},
            },
            wantErr: true,
            errContains: "cycle",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dag := NewTaskDAG()
            for taskID, deps := range tt.tasks {
                dag.AddTask(taskID, &Task{ID: taskID}, deps)
            }

            _, err := dag.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
            if err != nil && tt.errContains != "" {
                if !strings.Contains(err.Error(), tt.errContains) {
                    t.Errorf("Error message %q doesn't contain %q", err.Error(), tt.errContains)
                }
            }
        })
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| spf13/viper for all config | gookit/config for multi-file merge | 2024-2025 | Viper only supports single file per instance; gookit/config has true multi-file merging |
| Global mutex for resource locking | Keyed mutex (per-resource locks) | Pattern emerged ~2020 | 7-11x speedup when different goroutines access different resources (mapmutex benchmarks) |
| Manual goroutine cancellation with channels | context.Context for cancellation/timeout | Go 1.7+ (2016), standard since 1.13 | Context propagates cancellation through call stack; replaces error-prone channel patterns |
| DFS-based topological sort | Kahn's algorithm (BFS-based) | Preferred in Go ecosystem 2020+ | Kahn's algorithm naturally detects cycles via in-degree; DFS requires color tracking |

**Deprecated/outdated:**
- **mitchellh/go-homedir**: Use os.UserHomeDir() (stdlib since Go 1.12) or kirsle/configdir for full XDG support
- **sync.Map for all concurrent maps**: Use only for write-once/read-many or disjoint keys; regular map + mutex is often clearer and faster

## Open Questions

1. **Should workflow follow-ups create new DAG nodes or queue as separate plans?**
   - What we know: Phase 2 requirements say "orchestrator spawns follow-up agents per workflow config"
   - What's unclear: Are review/test tasks added to the SAME DAG (requiring re-validation) or treated as separate execution units?
   - Recommendation: Start with same DAG approach. Add workflow tasks to existing DAG, re-validate for cycles. Allows dependency tracking (e.g., "deploy" task depends on "test passed"). If complexity explodes, refactor to separate plan queues in Phase 3.

2. **How granular should file-level locking be?**
   - What we know: Requirements say "file-level resource locks prevent concurrent writes"
   - What's unclear: Does "file" mean exact path, or directory-level? What about file reads while another task writes?
   - Recommendation: Exact file path for writes. Two tasks writing different files in same directory should NOT block each other. For reads: don't lock (optimistic). If read/write conflicts emerge, upgrade to RWMutex pattern in Phase 3.

3. **Should default agent definitions be JSON files or Go constants?**
   - What we know: "Predefined roles ship as defaults" but format unspecified
   - What's unclear: Embedded JSON, Go structs with json tags, or separate default config file?
   - Recommendation: Go structs in internal/config/defaults.go that can be marshaled to JSON. Allows compile-time validation, easy testing, and users can still override via config files.

4. **Failure classification: what's the taxonomy?**
   - What we know: Requirements mention "hard/soft/skip" but no definitions
   - What's unclear: Concrete definitions and how they affect downstream tasks
   - Recommendation: Based on Airflow patterns (closest existing system):
     - **Hard failure**: Task failed, block ALL dependents
     - **Soft failure**: Task failed, but dependents CAN run (e.g., optional linting failed, but tests should still run)
     - **Skip**: Task intentionally not run (e.g., conditional), dependents treat as success
   - Implement as Task.FailureMode enum, checked in dependency resolution

## Sources

### Primary (HIGH confidence)
- [gammazero/toposort Go package](https://pkg.go.dev/github.com/gammazero/toposort) - API methods, cycle detection, Kahn's algorithm
- [gookit/config/v2 Go package](https://pkg.go.dev/github.com/gookit/config/v2) - Multi-file merge, override behavior, format support
- [Go sync package](https://pkg.go.dev/sync) - Mutex, RWMutex, Map, WaitGroup patterns
- [EagleChen/mapmutex](https://pkg.go.dev/github.com/EagleChen/mapmutex) - Keyed mutex API, benchmarks
- [kirsle/configdir](https://pkg.go.dev/github.com/kirsle/configdir) - Cross-platform config paths
- [Go context package](https://pkg.go.dev/context) - Cancellation, timeout, deadline patterns
- [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests) - Official testing patterns

### Secondary (MEDIUM confidence)
- [Kahn's Algorithm for Topological Sorting](https://www.geeksforgeeks.org/dsa/topological-sorting-indegree-based-solution/) - Algorithm explanation, verified with multiple sources
- [How to Use Mutex in Go (2026)](https://oneuptime.com/blog/post/2026-01-23-go-mutex/view) - Best practices, defer unlock, named fields
- [Table-Driven Tests in Go (Jan 2026)](https://medium.com/@mojimich2015/table-driven-tests-in-go-a-practical-guide-8135dcbc27ca) - Modern patterns
- [Gas Town Multi-Agent Framework](https://reading.torqsoftware.com/notes/software/ai-ml/agentic-coding/2026-01-15-gas-town-multi-agent-orchestration-framework/) - Multi-agent orchestration architecture in Go

### Tertiary (LOW confidence - architecture concepts, not implementation specifics)
- [Failure Handling in Apache Airflow DAGs](https://medium.com/@kopalgarg/failure-handling-in-apache-airflow-dags-6e20945859cd) - Conceptual model for failure classification; not Go-specific
- [Multi-Agent AI Systems (2026)](https://www.v7labs.com/blog/multi-agent-ai) - Orchestration patterns; language-agnostic

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries are published Go packages with official documentation and active maintenance
- Architecture: HIGH - Patterns verified from stdlib docs, official Go wiki, and package documentation
- Pitfalls: MEDIUM-HIGH - Derived from library docs, common Go concurrency pitfalls, and Airflow/DAG scheduler patterns; specific edge cases need validation during implementation
- Failure classification: LOW - Requirements mention it but provide no definitions; recommendation based on Airflow patterns but needs user validation

**Research date:** 2026-02-10
**Valid until:** ~30 days (stable domain; Go stdlib and core libraries change slowly)
