# Technology Stack

**Project:** Multi-Agent AI Orchestrator (Crush Fork)
**Domain:** Go-based TUI orchestrator for coordinating AI coding agents across multiple backends
**Researched:** 2026-02-10
**Overall Confidence:** HIGH

## Recommended Stack

### Core Framework & UI

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **charm.land/bubbletea/v2** | v2.0.0-rc.2 | TUI framework following Elm Architecture | Industry standard for Go TUIs. Powers 25k+ applications. v2 brings improved View API with consolidated view-related properties in single struct instead of scattered commands. Mature pattern for Model-Update-View architecture. |
| **charm.land/bubbles/v2** | v2.0.0-rc.1 | Pre-built TUI components (viewport, lists, etc.) | Official companion library for Bubble Tea. Viewport component essential for scrollable multi-pane layouts. Production-ready, battle-tested components that integrate seamlessly via message delegation. |
| **charm.land/lipgloss/v2** | v2.0.0-beta.3 | Terminal styling & layout | De facto standard for terminal layouts in Go. Critical best practice: account for borders in height calculations (subtract 2), use dynamic dimension tracking with Height()/Width() methods, never auto-wrap in bordered panels (always truncate). |

### LLM Abstraction & Agent Framework

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **charm.land/fantasy** | v0.7.1 | Multi-provider LLM abstraction | Charmbracelet's official LLM abstraction powering Crush. Unified API for OpenAI, Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, VertexAI. Supports openaicompat layer for any OpenAI-compatible endpoint (critical for local LLMs). Agent-oriented API with tool support. Zero external dependencies. |
| **github.com/openai/openai-go/v3** | v3.19.0 | Official OpenAI SDK | Official SDK released July 2024, now maintained by OpenAI. Supports Responses API (March 2025). Verify custom base URL support for OpenAI-compatible endpoints (documentation incomplete, check source). |
| **sashabaranov/go-openai** | Latest | Community OpenAI SDK with confirmed base URL support | Fallback option if official SDK lacks base URL customization. Proven `NewClientWithConfig` pattern: `config.BaseURL = "custom-url"` for OpenAI-compatible APIs. |
| **github.com/modelcontextprotocol/go-sdk** | v1.2.0 | Model Context Protocol client/server | Official MCP SDK maintained with Google. Enables standardized integration between LLMs and external data/tools. Essential for tool calling and context management in multi-agent systems. |

### Task Orchestration & Concurrency

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **golang.org/x/sync/errgroup** | v0.19.0 | Goroutine orchestration with error propagation | Idiomatic Go concurrency pattern for parallel agent execution. Provides context-based cancellation (one goroutine error cancels all via ctx.Done()), bounded concurrency via SetLimit(), clean error aggregation. Perfect for fan-out/fan-in agent workflows. |
| **AkihiroSuda/go-dag** | Latest | Minimalistic DAG scheduler | Lightweight DAG with concurrent execution. Minimal dependencies, focused API. Alternative: heimdalr/dag (faster, thread-safe, caches descendants/ancestors) if performance critical. Avoid Dagu (workflow engine with WebUI - too heavy, requires YAML configs). |

### Subprocess Management

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **os/exec** (stdlib) | Go 1.18+ | CLI subprocess execution | Standard library sufficient for Claude Code CLI/Codex CLI execution. Critical patterns: (1) Always use context.WithCancel/WithTimeout for cancellation, (2) Defer cancel() functions, (3) Start goroutines for stdout/stderr BEFORE Start(), (4) Never call Wait() until pipes consumed (prevents deadlock), (5) Security: As of Go 1.19, exec doesn't resolve programs via relative paths. |

### Database & Persistence

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **github.com/ncruces/go-sqlite3** | v0.30.5 | SQLite wrapper (CGo-free) | Wasm-based SQLite via wazero runtime. Zero C dependencies, cross-platform (Linux/macOS/Windows/BSD across amd64/arm64/riscv64). Provides both low-level SQLite API and database/sql driver. Includes JSON1, FTS5, incremental BLOB I/O, virtual tables. Tradeoff: higher memory usage vs CGo alternatives but eliminates C toolchain requirement. Crush uses this pattern. |
| **github.com/pressly/goose/v3** | v3.26.0 | Database migrations | Industry standard for Go migrations. Supports both SQL and Go migrations, embedded migration files, rollback support. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **charm.land/log/v2** | v2.0.0 | Structured logging with beautiful terminal output | User-facing logs. Unlike Zap/Zerolog (machine-optimized JSON), Charm Log prioritizes human readability with colorful output, icons, spacing. Supports TextFormatter (console), JSONFormatter, LogfmtFormatter (production). Creates sub-loggers with log.With(). |
| **charm.land/glamour/v2** | v2.0.0 | Markdown rendering in terminal | Rendering LLM responses with formatting. |
| **charm.land/catwalk** | v0.17.1 | LLM inference provider collection | Lower-level than Fantasy. Use if custom provider abstraction needed. |
| **golang.org/x/sys** | Latest | Low-level system calls | Subprocess signal handling, terminal control. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **sqlc** | SQL code generation | Crush uses this pattern (sqlc.yaml in repo). Type-safe SQL queries. |
| **golangci-lint** | Linting | Standard linter aggregator. Crush uses .golangci.yml config. |

## Installation

```bash
# Core framework
go get charm.land/bubbletea/v2@v2.0.0-rc.2
go get charm.land/bubbles/v2@v2.0.0-rc.1
go get charm.land/lipgloss/v2@v2.0.0-beta.3

# LLM abstraction
go get charm.land/fantasy@v0.7.1
go get github.com/modelcontextprotocol/go-sdk@v1.2.0

# Choose one OpenAI client:
go get github.com/openai/openai-go/v3@v3.19.0
# OR (if base URL customization confirmed missing from official SDK)
go get github.com/sashabaranov/go-openai@latest

# Orchestration
go get golang.org/x/sync@v0.19.0
go get github.com/AkihiroSuda/go-dag@latest

# Database
go get github.com/ncruces/go-sqlite3@v0.30.5
go get github.com/ncruces/go-sqlite3/driver  # database/sql driver
go get github.com/ncruces/go-sqlite3/embed   # embedded SQLite
go get github.com/pressly/goose/v3@v3.26.0

# Supporting
go get charm.land/log/v2@v2.0.0
go get charm.land/glamour/v2@v2.0.0
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative | Confidence |
|-------------|-------------|-------------------------|------------|
| **charm.land/fantasy** | Direct provider SDKs (openai-go, anthropic-sdk-go) | Single-provider projects where abstraction overhead unnecessary | HIGH |
| **AkihiroSuda/go-dag** | heimdalr/dag | Performance critical, need caching of descendants/ancestors | MEDIUM |
| **AkihiroSuda/go-dag** | Dagu workflow engine | Need WebUI, YAML-based workflows, persistent scheduling, production ops features (retries, monitoring, RBAC) | HIGH |
| **errgroup** | Manual WaitGroup + channels | Simple parallel execution without error propagation or context cancellation | HIGH |
| **ncruces/go-sqlite3** | modernc.org/sqlite (also CGo-free) | Alternative Wasm approach, check performance benchmarks for specific workload | LOW (insufficient comparison data) |
| **os/exec** | go-rillas/subprocess, lxd/subprocess | Need Python-like subprocess API with richer abstractions | MEDIUM |
| **Bubble Tea v2** | Bubble Tea v1 (github.com/charmbracelet/bubbletea) | Avoid v1 migrations if possible. v2 breaking changes: import path charm.land/*, View() returns View struct not string, message types converted from aliases to structs | HIGH |

## What NOT to Use

| Avoid | Why | Use Instead | Confidence |
|-------|-----|-------------|------------|
| **Google Wire** | Compile-time DI overkill for this project. Crush doesn't use it. Manual DI or Uber Fx (if complex lifecycle needed) more appropriate. | Manual constructor functions, or Uber Fx if need lifecycle hooks (OnStart/OnStop) | HIGH |
| **Dagu** (for in-process DAG) | Heavyweight workflow engine designed for persistent scheduling, WebUI, YAML configs. Introduces unnecessary complexity for in-process task graph. | AkihiroSuda/go-dag or heimdalr/dag for programmatic DAG | HIGH |
| **log/slog** (stdlib) | While standard as of Go 1.21, lacks the visual appeal critical for TUI applications. Charm Log provides better UX. | charm.land/log/v2 for user-facing logs, slog for machine logs if needed | MEDIUM |
| **Zap/Zerolog** (for TUI logs) | Optimized for machine parsing (JSON), not human readability. Wrong tool for CLI/TUI user feedback. | charm.land/log/v2 | HIGH |

## Stack Patterns by Variant

### Pattern 1: Official OpenAI SDK with Base URL Support
**If** official openai-go v3 supports custom base URLs:
- Use `github.com/openai/openai-go/v3` for all OpenAI-compatible endpoints
- Fantasy wraps this for multi-provider abstraction
- **Verify:** Check v3 RequestOptions/DefaultClientOptions for base URL config

### Pattern 2: Community SDK Fallback
**If** official SDK lacks base URL customization:
- Use `sashabaranov/go-openai` with `NewClientWithConfig(config)` pattern
- Set `config.BaseURL` for local LLMs via OpenAI-compatible API
- Fantasy may handle this internally, verify Fantasy v0.7.1 openaicompat implementation

### Pattern 3: Multi-Pane TUI Layout (Lipgloss + Bubbles)
**For split-pane agent output:**
1. Each pane = separate Model with Update()/View()
2. Parent orchestrator model composes pane models
3. Viewport component (bubbles/viewport) for scrollable content
4. Lipgloss for layout: use Height()/Width() for dynamic sizing, subtract 2 for borders, truncate don't wrap
5. Focus system: delegate KeyMsg to active pane only
6. Reference: [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/), [john-marinelli/panes component](https://github.com/john-marinelli/panes)

### Pattern 4: DAG Task Execution with Context Cancellation
```go
// Use errgroup for parallel agent tasks with cancellation
g, ctx := errgroup.WithContext(parentCtx)
g.SetLimit(maxConcurrentAgents) // Bounded concurrency

for _, task := range dag.GetReadyTasks() {
    task := task // Capture loop variable
    g.Go(func() error {
        return executeAgent(ctx, task)
    })
}

if err := g.Wait(); err != nil {
    // One agent failed, ctx already cancelled for others
    return err
}
```

### Pattern 5: Subprocess Management with Proper Cleanup
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel() // Always defer cancel

cmd := exec.CommandContext(ctx, "claude-code", "--prompt", prompt)

stdout, _ := cmd.StdoutPipe()
stderr, _ := cmd.StderrPipe()

// Start goroutines BEFORE Start() to prevent deadlock
go func() { io.Copy(os.Stdout, stdout) }()
go func() { io.Copy(os.Stderr, stderr) }()

if err := cmd.Start(); err != nil {
    return err
}

// Wait blocks until pipes fully consumed
if err := cmd.Wait(); err != nil {
    // Check if context cancelled vs actual error
    if ctx.Err() == context.DeadlineExceeded {
        return fmt.Errorf("agent timeout")
    }
    return err
}
```

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| **bubbletea/v2** | bubbles/v2, lipgloss/v2 | All v2 beta/rc versions in Charm ecosystem designed to work together. Import paths changed to charm.land/* in v2. |
| **fantasy v0.7.1** | openai-go/v2 (v2.7.1 in Crush) | Crush uses openai-go v2, but v3 is latest. Verify Fantasy compatibility with v3 before upgrading. |
| **ncruces/go-sqlite3** | Go 1.18+ | Requires wazero Wasm runtime, works across all platforms. Not at v1.0 yet (pre-stable). |
| **errgroup** | Go 1.18+ | Part of golang.org/x/sync, stable API. |

## Architecture Decisions

### Why Charm Ecosystem Over Alternatives?
1. **Proven at scale:** Powers Crush (Charm's own agent), 25k+ apps
2. **Cohesive design:** bubbletea/bubbles/lipgloss/fantasy designed to work together
3. **Active maintenance:** v2 releases in Nov 2024-Jan 2025, Fantasy updated Feb 9, 2026
4. **Domain fit:** TUI-first architecture matches terminal-based orchestrator requirements

### Why Fantasy Over Direct Provider SDKs?
1. **Multi-backend requirement:** Project needs Claude (Anthropic), Codex (OpenAI), local LLMs
2. **Consistent API:** Single interface reduces coupling, easier to add providers
3. **OpenAI-compatible layer:** Fantasy's openaicompat handles local LLMs via standard interface
4. **Crush precedent:** Forking Crush means inheriting Fantasy abstraction, maintain consistency

### Why errgroup Over Custom DAG Scheduler?
1. **Complementary tools:** errgroup handles parallel execution + cancellation, DAG handles dependency ordering
2. **Separation of concerns:** DAG determines task order, errgroup executes ready tasks concurrently
3. **Idiomatic Go:** errgroup is standard x/sync pattern, well-understood by Go developers

### Why CGo-free SQLite (ncruces)?
1. **Cross-compilation:** No C toolchain needed, simplifies builds across platforms
2. **Crush compatibility:** Forking Crush means maintaining similar dependency profile
3. **Sufficient performance:** Wasm overhead acceptable for metadata storage (not performance critical path)

## Open Questions & Validation Needed

| Question | Priority | How to Resolve |
|----------|----------|----------------|
| Does openai-go/v3 support custom base URLs? | HIGH | Check pkg.go.dev docs, review option.With* functions, or test with local LLM endpoint |
| Fantasy v0.7.1 compatibility with openai-go v3? | HIGH | Crush uses v2.7.1, check Fantasy release notes for v3 support |
| Performance: ncruces vs modernc.org/sqlite? | MEDIUM | Benchmark both with realistic workload (agent metadata writes) |
| heimdalr/dag vs AkihiroSuda/go-dag for task graph? | MEDIUM | Prototype both, evaluate caching benefits for complex dependency graphs |
| Bubble Tea v2 stability timeline? | LOW | Monitor releases, v2.0.0 final expected soon given RC status |

## Confidence Assessment

| Technology Area | Confidence | Rationale |
|----------------|------------|-----------|
| **TUI Framework (Bubble Tea/Bubbles/Lipgloss)** | HIGH | Official Charm stack, proven in Crush, extensive documentation, active development |
| **LLM Abstraction (Fantasy)** | HIGH | Crush uses this, official Charm library, supports required providers |
| **OpenAI SDK Choice** | MEDIUM | Official v3 SDK exists but base URL support unverified in docs, sashabaranov fallback confirmed working |
| **Concurrency (errgroup)** | HIGH | Standard x/sync pattern, well-documented, idiomatic Go |
| **DAG Libraries** | MEDIUM | Multiple options (AkihiroSuda, heimdalr), no clear winner, need prototyping |
| **Subprocess Management** | HIGH | Standard library os/exec sufficient, patterns well-established |
| **Database (SQLite)** | HIGH | Crush uses ncruces pattern, CGo-free benefits clear |
| **MCP Integration** | MEDIUM | Official SDK exists, maintained with Google, but integration patterns with Fantasy unclear |

## Sources

### High Confidence (Official/Context7)
- [Bubble Tea GitHub](https://github.com/charmbracelet/bubbletea) — Architecture, v2 breaking changes
- [Bubbles viewport package](https://pkg.go.dev/github.com/charmbracelet/bubbles/viewport) — Multi-pane component API
- [Fantasy GitHub](https://github.com/charmbracelet/fantasy) — Multi-provider LLM abstraction
- [Crush go.mod](https://github.com/charmbracelet/crush/blob/main/go.mod) — Actual dependencies in production
- [ncruces/go-sqlite3 pkg.go.dev](https://pkg.go.dev/github.com/ncruces/go-sqlite3) — CGo-free SQLite features
- [errgroup pkg.go.dev](https://pkg.go.dev/golang.org/x/sync/errgroup) — Goroutine orchestration API
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) — Official SDK maintained with Google

### Medium Confidence (Official Docs + Web Search Verification)
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) — Best practices for layouts
- [sashabaranov/go-openai custom base URL](https://github.com/sashabaranov/go-openai/blob/master/config.go) — Verified NewClientWithConfig pattern
- [Dagu features](https://github.com/dagu-org/dagu) — Workflow engine capabilities
- [Go context best practices](https://go.dev/blog/context) — Official Go blog on cancellation
- [Logging libraries comparison](https://betterstack.com/community/guides/logging/best-golang-logging-libraries/) — 2026 landscape

### Low Confidence (Web Search Only, Needs Verification)
- openai-go/v3 custom base URL support — Documentation incomplete, needs source code review
- Performance comparison ncruces vs modernc.org/sqlite — No benchmarks found
- Fantasy v0.7.1 compatibility with openai-go v3 — Crush uses v2, unclear if v3 tested

---
*Stack research for: Multi-Agent AI Orchestrator (Crush Fork)*
*Researched: 2026-02-10*
*Overall Confidence: HIGH*
