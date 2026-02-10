# Architecture Research

**Domain:** Multi-Agent AI Orchestration System
**Researched:** 2026-02-10
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         TUI Layer (Bubble Tea)                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │  Main View  │  │  Split Pane │  │  Status Bar │                 │
│  │  Coordinator│  │  Agent Views│  │  Metrics    │                 │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                 │
│         │                │                │                         │
│         └────────────────┴────────────────┘                         │
│                          │                                          │
├──────────────────────────┴──────────────────────────────────────────┤
│                     Orchestration Layer                             │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │                  Coordinator (Hub)                        │       │
│  │  - Task decomposition (DAG generation)                    │       │
│  │  - Agent lifecycle management                             │       │
│  │  - Dependency resolution                                  │       │
│  │  - Result aggregation                                     │       │
│  └──┬────────────┬────────────┬────────────┬────────────────┘       │
│     │            │            │            │                        │
│  ┌──▼──┐      ┌──▼──┐      ┌──▼──┐      ┌──▼──┐                    │
│  │Agent│      │Agent│      │Agent│      │Agent│                    │
│  │ 1   │      │ 2   │      │ 3   │      │ N   │                    │
│  └──┬──┘      └──┬──┘      └──┬──┘      └──┬──┘                    │
│     │            │            │            │                        │
├─────┴────────────┴────────────┴────────────┴────────────────────────┤
│                     Backend Abstraction Layer                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │  In-Process │  │  Subprocess │  │   Network   │                 │
│  │  (Fantasy)  │  │  (Claude    │  │  (API/MCP)  │                 │
│  │             │  │   Code CLI) │  │             │                 │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                 │
│         │                │                │                         │
├─────────┴────────────────┴────────────────┴─────────────────────────┤
│                         Execution Layer                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │  Go Runtime  │  │  OS Process  │  │  HTTP/JSON   │              │
│  │  Goroutines  │  │  Management  │  │  RPC Client  │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
├─────────────────────────────────────────────────────────────────────┤
│                         State Management                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │  Task Graph  │  │ Conversation │  │   Metrics &  │              │
│  │     DAG      │  │   History    │  │   Telemetry  │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **Coordinator** | Hub for task decomposition, agent selection, DAG scheduling, result aggregation | Hub-and-spoke pattern with work-stealing scheduler |
| **Agent** | Executes specific tasks, manages conversation state, tool execution | Interface-based abstraction over multiple backends |
| **Backend Adapter** | Translates between agent interface and specific LLM provider/CLI | Adapter pattern (in-process, subprocess, network) |
| **Task Scheduler** | Resolves dependencies, schedules parallel execution, manages work queue | DAG-based with topological sort and goroutine pool |
| **Subprocess Manager** | Launches/monitors CLI subprocesses, handles stdin/stdout streaming, JSON-RPC protocol | Process lifecycle with bidirectional communication |
| **State Store** | Persists conversation history, task results, agent metrics | In-memory with optional persistence layer |
| **TUI Manager** | Renders split-pane views, routes user input, updates agent displays | Bubble Tea MUV (Model-Update-View) architecture |
| **Event Bus** | Pub/sub for agent events, progress updates, inter-agent messaging | Channel-based or library like Watermill |

## Recommended Project Structure

```
orchestrator/
├── cmd/
│   └── orchestrator/       # Main entry point
│       └── main.go
├── internal/
│   ├── coordinator/        # Orchestration logic
│   │   ├── coordinator.go  # Hub implementation
│   │   ├── scheduler.go    # DAG scheduler
│   │   └── graph.go        # Task dependency graph
│   ├── agent/              # Agent abstraction
│   │   ├── agent.go        # Agent interface
│   │   ├── session.go      # Conversation state
│   │   └── registry.go     # Agent factory/registry
│   ├── backend/            # Backend adapters
│   │   ├── backend.go      # Backend interface
│   │   ├── fantasy.go      # In-process (Fantasy LLM)
│   │   ├── subprocess.go   # CLI subprocess wrapper
│   │   └── network.go      # Network API client
│   ├── subprocess/         # Process management
│   │   ├── manager.go      # Process lifecycle
│   │   ├── stdio.go        # stdin/stdout streaming
│   │   └── jsonrpc.go      # JSON-RPC protocol
│   ├── state/              # State management
│   │   ├── store.go        # State storage
│   │   ├── history.go      # Conversation history
│   │   └── metrics.go      # Performance metrics
│   ├── events/             # Event bus
│   │   ├── bus.go          # Pub/sub implementation
│   │   └── types.go        # Event definitions
│   ├── tui/                # Terminal UI
│   │   ├── app.go          # Bubble Tea app
│   │   ├── coordinator_view.go  # Main view
│   │   ├── agent_view.go   # Split pane agent views
│   │   ├── layout.go       # Layout manager
│   │   └── styles.go       # Lipgloss styles
│   └── tools/              # Shared tool system
│       ├── tool.go         # Tool interface (from Crush)
│       └── registry.go     # Tool registry
├── pkg/
│   └── config/             # Public configuration types
│       └── config.go
└── .planning/
    └── research/           # This file
```

### Structure Rationale

- **`coordinator/`**: Core orchestration logic isolated from UI and backends. Hub-and-spoke pattern with DAG scheduling for parallel task execution. Separates graph construction from scheduling for testability.

- **`agent/`**: Interface-based design allows multiple backend implementations. Session management handles multi-turn conversation state independently of backend type.

- **`backend/`**: Adapter pattern abstracts LLM provider differences. Three primary adapters: in-process (Fantasy), subprocess (Claude Code CLI), network (API/MCP). Each adapter implements the same interface for uniform agent interaction.

- **`subprocess/`**: Dedicated package for complex subprocess management. Handles bidirectional JSON-RPC communication over stdin/stdout. Critical for external CLI integration (Claude Code, Codex).

- **`state/`**: Centralized state management prevents inconsistency across concurrent agents. Conversation history storage enables context window management. Metrics collection for observability.

- **`events/`**: Decouples components via pub/sub pattern. Agents publish progress events, TUI subscribes for updates. Enables horizontal scaling later (replace channels with message broker).

- **`tui/`**: Bubble Tea MUV architecture. Layout manager handles dynamic split-pane resizing. Each agent gets dedicated view component with independent state.

- **`tools/`**: Reuses Crush's existing tool system. Shared registry allows different agents to access same tools with different permissions.

## Architectural Patterns

### Pattern 1: Hub-and-Spoke Orchestration

**What:** Central coordinator (hub) manages specialized agents (spokes). Coordinator decomposes tasks into DAG, assigns to agents, aggregates results.

**When to use:** Default pattern for most multi-agent systems. Provides governance, observability, predictable control flow. Suitable when reasoning transparency and traceability are critical.

**Trade-offs:**
- **Pros:** Strong central control, easier monitoring, consistent decision-making, audit-friendly
- **Cons:** Single point of failure, potential bottleneck, head-of-line blocking

**Example:**
```go
type Coordinator struct {
    agents   map[string]Agent
    scheduler *DAGScheduler
    eventBus *EventBus
}

func (c *Coordinator) Execute(task Task) (Result, error) {
    // Decompose into DAG
    dag := c.decompose(task)

    // Schedule parallel execution
    results := c.scheduler.Execute(dag, c.agents)

    // Aggregate results
    return c.aggregate(results)
}
```

**Scaling strategy:** Bake in ability to fan out, shard, and delegate coordination as traffic grows. Use work-stealing for load balancing across agents.

### Pattern 2: DAG-Based Task Scheduling

**What:** Represent task dependencies as directed acyclic graph. Topological sort determines execution order. Nodes = tasks, edges = dependencies. Enables maximum parallelism while respecting dependencies.

**When to use:** Complex workflows with parallel execution opportunities. When tasks have clear dependency relationships. Need to maximize throughput without violating constraints.

**Trade-offs:**
- **Pros:** Maximum parallelism, clear dependency visualization, automatic deadlock prevention (acyclic)
- **Cons:** Overhead of graph construction, complexity of dynamic task generation, requires careful error propagation

**Example:**
```go
type TaskNode struct {
    ID       string
    Task     Task
    Agent    string
    Children []*TaskNode
}

type DAGScheduler struct {
    workerPool chan struct{} // Limit concurrent goroutines
}

func (s *DAGScheduler) Execute(dag *TaskNode, agents map[string]Agent) Results {
    results := make(chan Result)
    var wg sync.WaitGroup

    // Topological execution with work-stealing
    s.executeNode(dag, agents, results, &wg)

    wg.Wait()
    close(results)

    return collectResults(results)
}

func (s *DAGScheduler) executeNode(node *TaskNode, agents map[string]Agent,
                                     results chan<- Result, wg *sync.WaitGroup) {
    wg.Add(1)
    go func() {
        defer wg.Done()

        // Execute task
        s.workerPool <- struct{}{} // Acquire worker slot
        result := agents[node.Agent].Execute(node.Task)
        <-s.workerPool // Release worker slot

        results <- result

        // Execute children in parallel
        for _, child := range node.Children {
            s.executeNode(child, agents, results, wg)
        }
    }()
}
```

### Pattern 3: Adapter-Based Backend Abstraction

**What:** Define common Agent interface. Implement adapters for each backend type (in-process LLM, subprocess CLI, network API). Agents use adapters transparently.

**When to use:** Multiple LLM providers or execution environments. Need to swap backends without changing orchestration logic. Testing with mock implementations.

**Trade-offs:**
- **Pros:** Decoupling from providers, easy testing, simple provider switching, reduced vendor lock-in
- **Cons:** Lowest-common-denominator interface, adapter complexity for provider-specific features

**Example:**
```go
// Agent interface (unchanged from Crush)
type Agent interface {
    Execute(ctx context.Context, task Task) (Result, error)
    Stream(ctx context.Context, task Task) (<-chan Event, error)
}

// Backend interface
type Backend interface {
    Send(ctx context.Context, messages []Message) (Response, error)
    StreamSend(ctx context.Context, messages []Message) (<-chan Token, error)
}

// In-process adapter (Fantasy)
type FantasyBackend struct {
    client *fantasy.Client
    model  string
}

func (b *FantasyBackend) Send(ctx context.Context, msgs []Message) (Response, error) {
    return b.client.Complete(ctx, b.model, msgs)
}

// Subprocess adapter (Claude Code CLI)
type SubprocessBackend struct {
    manager *subprocess.Manager
    process *subprocess.Process
}

func (b *SubprocessBackend) Send(ctx context.Context, msgs []Message) (Response, error) {
    // JSON-RPC over stdin/stdout
    return b.process.Call(ctx, "chat.complete", msgs)
}

// Network adapter (API/MCP)
type NetworkBackend struct {
    client *http.Client
    baseURL string
}

func (b *NetworkBackend) Send(ctx context.Context, msgs []Message) (Response, error) {
    // HTTP POST with JSON payload
    return b.client.Post(b.baseURL, msgs)
}

// SessionAgent wraps backend with conversation state
type SessionAgent struct {
    backend Backend
    history []Message
    tools   []Tool
}

func (a *SessionAgent) Execute(ctx context.Context, task Task) (Result, error) {
    // Add task to history
    a.history = append(a.history, task.ToMessage())

    // Send to backend
    response, err := a.backend.Send(ctx, a.history)

    // Update history
    a.history = append(a.history, response.ToMessage())

    return response.ToResult(), err
}
```

### Pattern 4: Subprocess JSON-RPC Communication

**What:** Launch external CLI as subprocess. Communicate via JSON-RPC 2.0 over stdin/stdout. Bidirectional protocol allows requests, responses, notifications, and subscriptions.

**When to use:** Integrating external tools like Claude Code, Codex. Tools only available as CLI. Need process isolation or sandboxing.

**Trade-offs:**
- **Pros:** Process isolation, language-agnostic, reuses existing CLIs, sandboxing
- **Cons:** Startup overhead, serialization cost, process management complexity, harder debugging

**Example:**
```go
type JSONRPCProcess struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    reqID  atomic.Int64
    pending map[int64]chan Response
    mu     sync.RWMutex
}

func (p *JSONRPCProcess) Start(command string, args ...string) error {
    p.cmd = exec.Command(command, args...)

    // Setup pipes
    stdin, _ := p.cmd.StdinPipe()
    stdout, _ := p.cmd.StdoutPipe()
    p.stdin = stdin
    p.stdout = stdout

    // Start reading responses
    go p.readLoop()

    return p.cmd.Start()
}

func (p *JSONRPCProcess) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
    id := p.reqID.Add(1)

    // Create response channel
    responseChan := make(chan Response, 1)
    p.mu.Lock()
    p.pending[id] = responseChan
    p.mu.Unlock()

    // Send request
    req := JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      id,
        Method:  method,
        Params:  params,
    }

    if err := json.NewEncoder(p.stdin).Encode(req); err != nil {
        return nil, err
    }

    // Wait for response
    select {
    case resp := <-responseChan:
        if resp.Error != nil {
            return nil, resp.Error
        }
        return resp.Result, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (p *JSONRPCProcess) readLoop() {
    decoder := json.NewDecoder(p.stdout)
    for {
        var resp JSONRPCResponse
        if err := decoder.Decode(&resp); err != nil {
            return // Process terminated
        }

        // Route response to waiting caller
        p.mu.RLock()
        ch := p.pending[resp.ID]
        p.mu.RUnlock()

        if ch != nil {
            ch <- resp
            p.mu.Lock()
            delete(p.pending, resp.ID)
            p.mu.Unlock()
        }
    }
}
```

### Pattern 5: Event-Driven Progress Updates

**What:** Agents publish events to event bus. TUI subscribes to events and updates views. Decouples event producers from consumers.

**When to use:** Multiple consumers need agent updates (TUI, metrics, logging). Agents should not know about UI. Need to add observers without modifying agents.

**Trade-offs:**
- **Pros:** Decoupling, flexible subscription, easy to add observers, testable
- **Cons:** Ordering guarantees harder, debugging event flow, potential memory leaks from subscriptions

**Example:**
```go
type EventBus struct {
    subscribers map[string][]chan Event
    mu          sync.RWMutex
}

func (b *EventBus) Subscribe(topic string) <-chan Event {
    ch := make(chan Event, 100) // Buffered to prevent blocking

    b.mu.Lock()
    b.subscribers[topic] = append(b.subscribers[topic], ch)
    b.mu.Unlock()

    return ch
}

func (b *EventBus) Publish(topic string, event Event) {
    b.mu.RLock()
    subscribers := b.subscribers[topic]
    b.mu.RUnlock()

    for _, ch := range subscribers {
        select {
        case ch <- event:
        default:
            // Drop event if subscriber can't keep up (non-blocking)
        }
    }
}

// Agent publishes progress
func (a *Agent) Execute(ctx context.Context, task Task) (Result, error) {
    a.eventBus.Publish("agent."+a.ID, Event{Type: "started", Task: task})

    result, err := a.backend.Send(ctx, task.ToMessages())

    a.eventBus.Publish("agent."+a.ID, Event{Type: "completed", Result: result})

    return result, err
}

// TUI subscribes to all agents
func (m *Model) Init() tea.Cmd {
    return func() tea.Msg {
        for agentID := range m.agents {
            events := m.eventBus.Subscribe("agent." + agentID)
            go func(id string, ch <-chan Event) {
                for evt := range ch {
                    m.program.Send(AgentEventMsg{AgentID: id, Event: evt})
                }
            }(agentID, events)
        }
        return nil
    }
}
```

### Pattern 6: Work-Stealing Scheduler

**What:** Each processor (goroutine) has local task queue. Idle processors steal tasks from other queues. Go runtime uses this pattern internally.

**When to use:** High-concurrency scenarios with variable task durations. Want automatic load balancing without manual sharding. Prevent idle workers while others are overloaded.

**Trade-offs:**
- **Pros:** Automatic load balancing, no idle workers, good cache locality (local queue first)
- **Cons:** Lock contention on steals, complexity of deque implementation

**Example:**
```go
type WorkStealingScheduler struct {
    queues    []chan Task // One queue per worker
    workers   int
    running   atomic.Bool
}

func NewWorkStealingScheduler(workers int) *WorkStealingScheduler {
    queues := make([]chan Task, workers)
    for i := range queues {
        queues[i] = make(chan Task, 256) // Local queue
    }
    return &WorkStealingScheduler{queues: queues, workers: workers}
}

func (s *WorkStealingScheduler) Start(agents map[string]Agent) {
    s.running.Store(true)

    for i := 0; i < s.workers; i++ {
        go s.worker(i, agents)
    }
}

func (s *WorkStealingScheduler) worker(id int, agents map[string]Agent) {
    localQueue := s.queues[id]

    for s.running.Load() {
        select {
        case task := <-localQueue:
            // Execute from local queue
            agents[task.AgentID].Execute(context.Background(), task)

        default:
            // Local queue empty, try stealing
            if task := s.steal(id); task != nil {
                agents[task.AgentID].Execute(context.Background(), *task)
            } else {
                time.Sleep(time.Millisecond) // Back off
            }
        }
    }
}

func (s *WorkStealingScheduler) steal(thiefID int) *Task {
    // Try stealing from random victim
    victimID := rand.Intn(s.workers)
    if victimID == thiefID {
        return nil
    }

    select {
    case task := <-s.queues[victimID]:
        return &task
    default:
        return nil
    }
}

func (s *WorkStealingScheduler) Submit(task Task) {
    // Hash agent ID to worker queue for locality
    workerID := hash(task.AgentID) % s.workers
    s.queues[workerID] <- task
}
```

## Data Flow

### Request Flow

```
User Input (TUI)
    ↓
Coordinator.HandleRequest()
    ↓
Task Decomposition (LLM-powered reasoning)
    ↓
DAG Construction (nodes = subtasks, edges = dependencies)
    ↓
DAG Scheduler (topological sort + work-stealing)
    ↓
Agent Pool (parallel execution via goroutines)
    ↓
Backend Adapters (Fantasy / Subprocess / Network)
    ↓
LLM Execution (in-process, CLI, or API)
    ↓
Result Collection (channel aggregation)
    ↓
Result Aggregation (coordinator synthesizes)
    ↓
Event Publish (completion event)
    ↓
TUI Update (Bubble Tea Msg)
```

### Agent Communication Flow

```
Agent A (via backend)
    ↓
Result published to Event Bus (topic: "agent.A.completed")
    ↓
Coordinator subscribes to all agent events
    ↓
Coordinator checks DAG dependencies
    ↓
Unblocked tasks scheduled to Agent Pool
    ↓
Agent B receives dependent task
    ↓
Agent B accesses Agent A's result from State Store
```

### Subprocess Communication Flow

```
Coordinator
    ↓
SubprocessBackend.Send()
    ↓
JSON-RPC Request (method: "chat.complete", params: messages)
    ↓
stdin → Claude Code CLI process
    ↓
stdout → JSON-RPC Response stream
    ↓
Response Parser (line-by-line, buffered)
    ↓
Request/Response Correlation (match by ID)
    ↓
Result returned to SessionAgent
    ↓
History updated with assistant message
```

### State Management Flow

```
┌─────────────────────────────────────────────┐
│           Conversation History              │
│  ┌─────────────────────────────────────┐    │
│  │ Agent A: [msg1, msg2, msg3]         │    │
│  │ Agent B: [msg1, msg2]               │    │
│  │ Agent C: [msg1]                     │    │
│  └─────────────────────────────────────┘    │
│                                             │
│  SessionAgent → State Store (read history)  │
│  SessionAgent ← State Store (write msg)     │
│                                             │
│           Task Execution Graph              │
│  ┌─────────────────────────────────────┐    │
│  │ Node A: completed ✓                 │    │
│  │ Node B: in-progress ⟳               │    │
│  │ Node C: blocked (waiting for B)     │    │
│  └─────────────────────────────────────┘    │
│                                             │
│  Coordinator → State Store (update status)  │
│  TUI ← State Store (subscribe to changes)   │
└─────────────────────────────────────────────┘
```

### Key Data Flows

1. **Task Decomposition**: User request → Coordinator uses LLM to reason about task → Generates DAG with nodes and dependencies → Returns execution plan

2. **Parallel Execution**: DAG scheduler uses topological sort → Identifies tasks with no pending dependencies → Launches goroutines for parallel execution → Collects results via channels

3. **Context Management**: Agent retrieves conversation history from State Store → Appends new user message → Sends to backend → Receives response → Appends assistant message → Updates State Store

4. **Progress Streaming**: Agent publishes "started" event → TUI receives event via subscription → Updates agent view to "in-progress" → Agent publishes "token" events during streaming → TUI displays tokens in real-time → Agent publishes "completed" event → TUI updates view to "done"

5. **Inter-Agent Handoffs**: Agent A completes subtask → Result stored in State Store → DAG scheduler marks node complete → Dependent nodes become unblocked → Agent B retrieves Agent A's result from State Store → Agent B executes with context

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **1-5 agents** | Simple hub-and-spoke with goroutine-per-agent. In-memory state store. Single coordinator. No sharding needed. |
| **5-20 agents** | Add work-stealing scheduler to prevent bottlenecks. Implement circuit breakers for subprocess failures. Add retry logic with exponential backoff. Metrics collection for observability. |
| **20-100 agents** | Shard agents across multiple coordinator instances (consistent hashing). Replace in-memory state with persistent store (Redis/PostgreSQL). Add rate limiting per backend. Implement backpressure mechanisms. |
| **100+ agents** | Move to event-driven architecture (replace channels with Kafka/NATS). Distributed DAG execution (each coordinator handles subgraph). Agent registry service for discovery. Centralized observability platform. |

### Scaling Priorities

1. **First bottleneck: Coordinator saturation**
   - **Symptoms:** High latency on task decomposition, queue buildup, slow DAG generation
   - **Fix:** Cache common DAG patterns, add coordinator pool with load balancing, optimize LLM calls for decomposition (use smaller/faster models)

2. **Second bottleneck: State store contention**
   - **Symptoms:** Lock contention on history updates, slow reads/writes, memory pressure
   - **Fix:** Shard state by agent ID, use append-only event log pattern, move to persistent store with read replicas

3. **Third bottleneck: Subprocess overhead**
   - **Symptoms:** Slow startup times, high fork cost, process limit exhaustion
   - **Fix:** Process pool with keep-alive, replace subprocess with network API where possible, batch requests to same CLI instance

4. **Fourth bottleneck: TUI rendering performance**
   - **Symptoms:** Dropped frames, laggy input, high CPU on render
   - **Fix:** Throttle event updates (only render every N ms), use virtual scrolling for large agent lists, offload rendering to background goroutine

## Anti-Patterns

### Anti-Pattern 1: Bag of Agents (No Orchestration)

**What people do:** Create multiple agents without coordination. Let them communicate freely without structure. No task decomposition or dependency management.

**Why it's wrong:** Agents descend into circular logic or "hallucination loops" where they echo and validate each other's mistakes. Without orchestrator, failures increase silent drift. 17x higher error rate in production systems.

**Do this instead:** Use hub-and-spoke pattern with central coordinator. Define explicit handoff contracts (use structured outputs, JSON Schema). Implement orchestrator that monitors and validates outputs.

### Anti-Pattern 2: Prompt Entanglement

**What people do:** Embed agent coordination logic in system prompts. Use free-text instructions for handoffs. Rely on LLM to understand workflow structure.

**Why it's wrong:** Free-text handoffs are main source of context loss. LLMs struggle to maintain consistency across turns. Makes workflows impossible to debug or version.

**Do this instead:** Separate coordination logic from prompts. Use code for workflow structure (DAGs, state machines). Treat inter-agent transfers like public APIs with versioned contracts.

### Anti-Pattern 3: Premature Complexity

**What people do:** Build 10-agent system before validating single agent works. Use complex pattern when sequential orchestration suffices. Add agents without meaningful specialization.

**Why it's wrong:** Adds unnecessary coordination overhead. More failure modes. Harder to debug. Lower reliability.

**Do this instead:** Start with single agent. Add second agent only when clear specialization benefit. Prefer sequential → concurrent → hub-and-spoke → event-driven (in that order). Measure before adding complexity.

### Anti-Pattern 4: Shared Mutable State

**What people do:** Multiple agents read/write same data structures without synchronization. Pass entire conversation histories between agents. Store state in agent structs instead of centralized store.

**Why it's wrong:** Race conditions and data corruption. Transactionally inconsistent data. Hard to reason about agent interactions. Memory leaks from unbounded histories.

**Do this instead:** Use centralized State Store with proper locking. Pass immutable snapshots or specific fields (not entire history). Implement copy-on-write for shared data. Use channels for agent communication (Go principle: "Share memory by communicating").

### Anti-Pattern 5: Synchronous Subprocess Calls

**What people do:** Block goroutine waiting for subprocess response. No timeout handling. No circuit breaker for failing processes. Launch new process per request.

**Why it's wrong:** One slow/hung process blocks entire agent. No failure isolation. Process churn overhead. Resource exhaustion.

**Do this instead:** Async subprocess management with context cancellation. Implement timeouts on all subprocess calls. Use circuit breaker pattern (gobreaker library). Process pool with keep-alive. Graceful degradation when subprocess unavailable.

### Anti-Pattern 6: Unbounded Goroutines

**What people do:** Launch goroutine per task without limits. No worker pool. Assume Go can handle infinite concurrency.

**Why it's wrong:** OOM from too many goroutines. Scheduler overhead. Context switching thrashing. System becomes unresponsive.

**Do this instead:** Use worker pool with bounded goroutines (pattern from WorkStealingScheduler above). Implement backpressure (block task submission when queue full). Monitor goroutine count as health metric. Set GOMAXPROCS appropriately.

### Anti-Pattern 7: No Observability

**What people do:** No logging of agent interactions. No metrics on task duration. Can't trace request through agent pipeline. Silent failures.

**Why it's wrong:** Impossible to debug production issues. Can't identify bottlenecks. Don't know which agents are failing. Agent systems "fail quietly" without obvious errors.

**Do this instead:** Instrument all agent operations with structured logging. Track metrics (task duration, error rate, token usage per agent). Implement distributed tracing (correlation ID through agent pipeline). Publish events for audit trail. Dashboard for real-time monitoring.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| **Claude Code CLI** | Subprocess with JSON-RPC over stdin/stdout | Use buffered reader, line-by-line parsing. Handle process lifecycle (start, monitor, restart). Implement timeout (default 2min via context). |
| **Fantasy (charm.land/fantasy)** | In-process library | Direct function calls, no IPC overhead. Abstracts OpenAI, Anthropic, Google, AWS Bedrock, Azure, VertexAI. Use for simple agents. |
| **MCP Servers** | Network HTTP or stdio transport | Client-server architecture. MCP client in orchestrator, MCP server exposes tools/resources. JSON-RPC 2.0 protocol. Prefer stdio for local, SSE for remote. |
| **LLM APIs** | HTTP REST/streaming | Use http.Client with timeouts. Implement retry with exponential backoff. Circuit breaker for failing endpoints. Connection pooling. |
| **State Store** | Direct library (in-memory) or Redis (persistent) | Start with map[string][]Message in-memory. Move to Redis when scaling beyond single process. Use pub/sub for state change notifications. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| **Coordinator ↔ Agent** | Interface calls (sync) or channel messages (async) | Sync for request/response. Async for fire-and-forget tasks. Agent interface hides backend complexity. |
| **Agent ↔ Backend** | Backend interface (adapter pattern) | Backend abstracts transport (in-process, subprocess, network). Agent doesn't know backend type. |
| **Subprocess Manager ↔ CLI** | stdin/stdout pipes with JSON-RPC | Bidirectional protocol. Manager maintains correlation between requests/responses. Handle partial reads. |
| **Scheduler ↔ Worker Pool** | Go channels (buffered) | Scheduler enqueues tasks to channel. Workers dequeue and execute. Channel size = max queued tasks. |
| **Agent ↔ Event Bus** | Pub/sub via channels | Agent publishes events (fire-and-forget). Event Bus fans out to subscribers. Buffered channels prevent blocking. |
| **TUI ↔ Coordinator** | Bubble Tea Cmd/Msg pattern | TUI sends tea.Cmd to coordinator. Coordinator returns tea.Msg with results. All state changes via messages (MUV architecture). |
| **State Store ↔ Components** | Read/write API with mutex | Synchronous reads/writes with proper locking. Subscribe API for change notifications. Copy-on-read for safety. |

### Extending Crush Architecture

**Current Crush Components (Reuse):**
- `SessionAgent` interface → **Keep, extend with role metadata**
- `Coordinator` with `agents map[string]Agent` → **Keep, add scheduler field**
- Tool system → **Reuse as-is**
- Fantasy LLM abstraction → **Wrap in Backend adapter**
- Bubble Tea TUI → **Extend with split-pane layout**

**New Components (Add):**
- `DAGScheduler` → **New package `coordinator/`**
- `Backend` interface → **New package `backend/`** with Fantasy, Subprocess, Network adapters
- `SubprocessManager` → **New package `subprocess/`**
- `EventBus` → **New package `events/`**
- `StateStore` → **New package `state/`**
- Split-pane views → **Extend `tui/` with `layout.go` and `agent_view.go`**

**Migration Path:**
1. **Phase 1:** Extract Backend interface from SessionAgent. Create FantasyBackend adapter wrapping existing Fantasy code. No behavior change, just refactoring.

2. **Phase 2:** Add SubprocessBackend adapter. Implement JSON-RPC protocol. Test with single subprocess agent alongside existing Fantasy agents.

3. **Phase 3:** Add Scheduler to Coordinator. Initially run tasks sequentially (no parallelism). DAG with single path.

4. **Phase 4:** Implement parallel DAG execution with goroutine pool. Add work-stealing if needed.

5. **Phase 5:** Add EventBus. Agents publish progress events. Coordinator subscribes for monitoring.

6. **Phase 6:** Extend TUI with split-pane layout. One pane per active agent. Layout manager handles resizing.

7. **Phase 7:** Add StateStore for conversation persistence. Migrate in-memory history to store.

8. **Phase 8:** Add circuit breakers, retries, timeouts for resilience.

## Build Order Recommendations

**Dependencies determine build order:**

```
1. Backend Interface & Adapters
   ├── Required by: SessionAgent refactor
   └── Enables: Multiple agent types

2. EventBus
   ├── Required by: Progress monitoring
   └── Enables: Decoupled communication

3. StateStore
   ├── Required by: Conversation persistence
   └── Enables: Shared state across agents

4. SubprocessManager
   ├── Required by: CLI integration
   ├── Depends on: Backend Interface
   └── Enables: External tool support

5. DAG Scheduler
   ├── Required by: Parallel execution
   ├── Depends on: Backend Interface, StateStore
   └── Enables: Complex workflows

6. Extended TUI
   ├── Required by: Multi-agent visibility
   ├── Depends on: EventBus, Coordinator
   └── Enables: Split-pane monitoring

7. Resilience Patterns
   ├── Required by: Production readiness
   ├── Depends on: All above
   └── Enables: Circuit breakers, retries, timeouts
```

**Suggested Implementation Order:**
1. Backend abstraction (refactor existing Fantasy code)
2. Subprocess manager (enables Claude Code integration)
3. Event bus (enables progress monitoring)
4. State store (enables conversation persistence)
5. Basic DAG scheduler (sequential first, then parallel)
6. Split-pane TUI (visibility into agents)
7. Work-stealing scheduler (performance optimization)
8. Resilience patterns (production hardening)

**Each phase produces working system (no big-bang integration).**

## Sources

### Multi-Agent Orchestration Patterns
- [AI Agent Orchestration Patterns - Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)
- [Top 10+ Agentic Orchestration Frameworks & Tools in 2026](https://aimultiple.com/agentic-orchestration)
- [Choosing the right orchestration pattern for multi agent systems](https://www.kore.ai/blog/choosing-the-right-orchestration-pattern-for-multi-agent-systems)
- [AI Agent Orchestration in 2026: Coordination, Scale and Strategy](https://kanerika.com/blogs/ai-agent-orchestration/)
- [Choosing the Right Multi-Agent Architecture](https://blog.langchain.com/choosing-the-right-multi-agent-architecture/)

### Hub-and-Spoke vs Peer-to-Peer Topologies
- [Bot-to-Bot: Centralized (Hub-and-Spoke) Multi-Agent Topology — Part 2](https://medium.com/@ratneshyadav_26063/bot-to-bot-centralized-hub-and-spoke-multi-agent-topology-part-2-87b46ec7e1bc)
- [Hub & Spoke: The Operating System for AI-Enabled Enterprise Architecture](https://www.architectureandgovernance.com/artificial-intelligence/hub-spoke-the-operating-system-for-ai-enabled-enterprise-architecture/)

### DAG Task Scheduling
- [Multi-agent Reinforcement Learning-based Adaptive Heterogeneous DAG Scheduling](https://dl.acm.org/doi/10.1145/3610300)
- [Building Intelligent Agents with Dynamic DAGs: A Modular Approach to AI Design](https://medium.com/@alexfiorenza2012/building-intelligent-agents-with-dynamic-dags-a-modular-approach-to-ai-design-9f0cff8e550d)
- [TDAG: A Multi-Agent Framework based on Dynamic Task Decomposition and Agent Generation](https://arxiv.org/abs/2402.10178)

### Subprocess Management
- [Running Claude Code from Windows CLI: A Practical Guide](https://dstreefkerk.github.io/2026-01-running-claude-code-from-windows-cli/)
- [How to stream lines from stdout with subprocess](https://alexwlchan.net/til/2025/subprocess-line-by-line/)
- [Python Subprocess: Reading Stdout and Stderr Separately While Preserving Order with Asyncio](https://copyprogramming.com/howto/python-asyncio-subprocess-write-stdin-and-read-stdout-stderr-continuously)

### Multi-Turn Conversation State Management
- [Multi-agent Conversation Framework | AutoGen 0.2](https://microsoft.github.io/autogen/0.2/docs/Use-Cases/agent_chat/)
- [The Complete Guide to Managing Conversation History in Multi-Agent AI Systems](https://medium.com/@_Ankit_Malviya/the-complete-guide-to-managing-conversation-history-in-multi-agent-ai-systems-0e0d3cca6423)
- [Context Window Management: Strategies for Long-Context AI Agents and Chatbots](https://www.getmaxim.ai/articles/context-window-management-strategies-for-long-context-ai-agents-and-chatbots/)

### Bubble Tea TUI Architecture
- [GitHub - charmbracelet/bubbletea: A powerful little TUI framework](https://github.com/charmbracelet/bubbletea)
- [Shifoo - Multi View Interfaces in Bubble Tea](https://shi.foo/weblog/multi-view-interfaces-in-bubble-tea)
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)

### Framework Comparisons
- [CrewAI vs LangGraph vs AutoGen: Choosing the Right Multi-Agent AI Framework](https://www.datacamp.com/tutorial/crewai-vs-langgraph-vs-autogen)
- [Best AI Agent Frameworks in 2026: CrewAI vs. AutoGen vs. LangGraph](https://medium.com/@kia556867/best-ai-agent-frameworks-in-2026-crewai-vs-autogen-vs-langgraph-06d1fba2c220)
- [Agent Orchestration 2026: LangGraph, CrewAI & AutoGen Guide](https://iterathon.tech/blog/ai-agent-orchestration-frameworks-2026)

### Claude Code SDK
- [Run Claude Code programmatically - Claude Code Docs](https://code.claude.com/docs/en/headless)
- [Embedding Claude Code SDK in Applications](https://blog.bjdean.id.au/2025/11/embedding-claide-code-sdk-in-applications/)
- [Claude Agent SDK (Python) Learning Guide](https://redreamality.com/blog/claude-agent-sdk-python-/)

### Go Concurrency Patterns
- [Goroutines in Go: A Practical Guide to Concurrency](https://getstream.io/blog/goroutines-go-concurrency-guide/)
- [How to Use Goroutines and Channels for Concurrent Processing](https://oneuptime.com/blog/post/2026-01-07-go-goroutines-channels-concurrency/view)
- [Go Concurrency Patterns: Pipelines and cancellation](https://go.dev/blog/pipelines)

### Work-Stealing Scheduler
- [Building a Multithreaded Work-Stealing Task Scheduler in Go](https://medium.com/@nathanbcrocker/building-a-multithreaded-work-stealing-task-scheduler-in-go-843861b878be)
- [Go's work-stealing scheduler](https://rakyll.org/scheduler/)
- [Scheduling In Go : Part II - Go Scheduler](https://www.ardanlabs.com/blog/2018/08/scheduling-in-go-part2.html)

### Model Context Protocol
- [Specification - Model Context Protocol](https://modelcontextprotocol.io/specification/2025-11-25)
- [What Is the Model Context Protocol (MCP) and How It Works](https://www.descope.com/learn/post/mcp)
- [Getting Started With MCP Servers: A Technical Deep Dive](https://neo4j.com/blog/developer/model-context-protocol/)

### Charm Fantasy
- [GitHub - charmbracelet/fantasy: Build AI agents with Go](https://github.com/charmbracelet/fantasy)
- [AI and LLM Integration | charmbracelet/crush](https://deepwiki.com/charmbracelet/crush/4-ai-and-llm-integration)
- [DraganSr: GoLang powered AI tools: Charm Crush, Fantasy](https://blog.dragansr.com/2025/11/golang-powered-ai-tools-charm-crush.html)

### Event-Driven Architecture
- [GitHub - ThreeDotsLabs/watermill: Building event-driven applications the easy way in Go](https://github.com/ThreeDotsLabs/watermill)
- [Go for Event-Driven Architecture: Designing Pub/Sub Systems with NATS and Redis Streams](https://levelup.gitconnected.com/go-for-event-driven-architecture-designing-pub-sub-systems-with-nats-and-redis-streams-1adcd10b5fa1)
- [Golang Event-Driven Architecture Guide](https://blog.jealous.dev/mastering-event-driven-architecture-in-golang-comprehensive-insights)

### JSON-RPC in Go
- [jsonrpc2 package - golang.org/x/tools/internal/jsonrpc2](https://pkg.go.dev/golang.org/x/tools/internal/jsonrpc2)
- [GitHub - deinstapel/go-jsonrpc: Bidirectional JSON-RPC Library for Go](https://github.com/deinstapel/go-jsonrpc)
- [GitHub - cenkalti/rpc2: Bi-directional RPC in Go](https://github.com/cenkalti/rpc2)

### Anti-Patterns and Pitfalls
- [Anti-Patterns in Multi-Agent Gen AI Solutions: Enterprise Pitfalls and Best Practices](https://medium.com/@armankamran/anti-patterns-in-multi-agent-gen-ai-solutions-enterprise-pitfalls-and-best-practices-ea39118f3b70)
- [Why Your Multi-Agent System is Failing: Escaping the 17x Error Trap of the "Bag of Agents"](https://towardsdatascience.com/why-your-multi-agent-system-is-failing-escaping-the-17x-error-trap-of-the-bag-of-agents/)
- [Agent Systems Fail Quietly: Why Orchestration Matters More Than Intelligence](https://bnjam.dev/posts/agent-orchestration/agent-systems-fail-quietly.html)

### Resilience Patterns
- [GitHub - sony/gobreaker: Circuit Breaker implemented in Go](https://github.com/sony/gobreaker)
- [Resilient Go Microservices with Circuit Breakers](https://medium.com/@irem.gunay/resilient-go-microservices-with-circuit-breakers-4780c397e6f6)
- [Resilience Design Patterns: Retry, Fallback, Timeout](https://www.codecentric.de/en/knowledge-hub/blog/resilience-design-patterns-retry-fallback-timeout-circuit-breaker)

---
*Architecture research for: Multi-Agent AI Orchestration System*
*Researched: 2026-02-10*
