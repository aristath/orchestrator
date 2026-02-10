# Feature Research

**Domain:** Multi-Agent AI Coding Orchestrators
**Researched:** 2026-02-10
**Confidence:** MEDIUM

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Role-based agent definitions | Standard in CrewAI, MetaGPT, AutoGen; users expect configurable agent personas | MEDIUM | System prompts defining role, goal, backstory, tools. YAML config files are industry standard. |
| Task queue/DAG scheduling | Core coordination mechanism; LangGraph, Apache Airflow use DAGs for dependency management | HIGH | DAG enables parallel execution of independent tasks while respecting dependencies. Critical for efficient resource use. |
| Parallel agent execution | Ralph TUI, Clark demonstrate this is expected; users won't accept serial-only execution | HIGH | Requires git worktree isolation or similar. Users expect 4+ concurrent agents minimum. |
| Real-time TUI monitoring | Ralph TUI, Clark, TmuxCC establish TUI as standard for multi-agent visibility | MEDIUM | Split-pane view of all active agents. Vim-style navigation expected. Real-time logs and status updates. |
| Inter-agent message passing | Documented in A2A protocol, MCP; agents must communicate to coordinate | MEDIUM | JSON-RPC 2.0 messaging. Pub-sub and request-response patterns. Shared context mechanism. |
| Session/context persistence | OpenAI Agents SDK, Google ADK treat sessions as table stakes for coherent multi-turn interactions | MEDIUM | Store conversation history, agent decisions, and intermediate states. Must survive interruptions. |
| Error recovery with retry logic | Universal across frameworks; exponential backoff with jitter is standard pattern | MEDIUM | Wrap tool invocations with retry decorators. Circuit breakers for external services. Stateful recovery to resume after failure. |
| Backend abstraction layer | OpenCode supports 75+ models; users expect to swap OpenAI/Claude/local LLMs without code changes | MEDIUM | Provider-agnostic API. OpenAI-compatible endpoint format. LiteLLM integration pattern. |
| Agent status tracking | Users need to know: working, paused, failed, complete; standard in Clark, Ralph TUI | LOW | Per-agent state machine. Clear visual indicators in TUI. |
| Git integration for isolation | Ralph TUI and Clark use git worktrees; prevents conflicts in parallel execution | HIGH | One worktree per agent. Branch-per-task pattern. Merge strategy for completed work. |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Specialist-agent auto-review | Multi-model review panels (correctness, security, performance, observability agents) catch 40% more issues than single-pass | HIGH | Addresses 2026's "quality deficit" problem where agent output exceeds human review capacity. Run code through specialized models. |
| Hub-and-spoke architecture with full-plan visibility | Orchestrator holds complete plan while satellites execute sub-tasks; prevents coordination underspecification | MEDIUM | Differentiates from flat multi-agent where no one has full picture. Enables better conflict prevention. |
| Hierarchical memory (user/session/agent levels) | Mem0-style graph memory with cross-session recall; agents remember learnings from past projects | HIGH | Goes beyond session-only memory. Extracts facts, builds knowledge graph. Enables continuous improvement. |
| Dynamic agent spawning based on DAG | Create agents on-demand as tasks become ready; scales from 1 to N agents based on workload | MEDIUM | More efficient than fixed agent pool. Spawn coder when coding task ready, reviewer when code complete. |
| Multi-backend routing per agent | Route each agent to optimal backend (GPT-4 for planning, DeepSeek for coding, local for reviews) | MEDIUM | Cost optimization + performance tuning. Different tasks need different models. |
| "Land the plane" session cleanup | Beads pattern: structured end-of-session sweep ensuring all promises kept, deps resolved | LOW | Prevents abandoned work. Generates cleanup checklist. Human approval before archive. |
| Cross-session project memory | Agent recalls architectural decisions, coding patterns, past mistakes across sessions | HIGH | Uses git history + structured notes. Prevents repeating resolved issues. Builds institutional knowledge. |
| Quality gate orchestration | Automated gates: test coverage >70%, security scan, performance benchmarks before merge | MEDIUM | Multi-model reviews + automated testing loops. Configurable thresholds. Fails fast on violations. |
| Agent communication protocol standardization | MCP for tool connections + A2A for agent coordination in one system | MEDIUM | Interoperability differentiator. Agents from different frameworks can collaborate. |
| Visual DAG editor in TUI | Interactive task graph visualization with dependency editing, not just monitoring | HIGH | Makes complex workflows comprehensible. Drag-to-reorder priorities. Real-time execution overlay. |
| Smart task rebalancing | When one agent blocks, redistribute pending work to available agents | MEDIUM | Clark mentions this. Prevents idle agents. Requires task dependency analysis. |
| Context compression for long sessions | OpenAI Agents SDK's trimming + compression; maintains coherence without token explosion | MEDIUM | Prevents context overflow. Summarizes completed work. Retains critical decisions. |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Real-time collaboration on same file | "Why can't agents pair program?" sounds efficient | Race conditions, merge conflicts, coordination overhead destroys efficiency | Task-level parallelism with clear file ownership. Agents review each other's completed work, not edit simultaneously. |
| Unlimited agent spawning | "More agents = faster" seems logical | Coordination complexity grows O(n²). Most projects bottleneck at 4-6 agents per Cursor's hierarchical approach | Fixed role slots (2-3 workers, 1 planner, 1 judge). Spawn within limits based on workload. |
| Passing full conversation history between agents | Agents need context to coordinate | Token costs explode. Agents lose focus. Quality degrades. 2026 identified as key anti-pattern | Summarized task context + shared memory store. Agents read what they need, not everything. |
| Fully autonomous operation (no human gates) | "Set it and forget it" appeals to busy developers | 40% of agentic AI projects cancelled by 2027 due to unexpected risks. Silent failures compound | Human approval at critical gates: architecture decisions, security changes, deployments. "Proposal not effect" pattern. |
| Single massive orchestration graph | One graph to rule them all simplifies mental model | Becomes unmaintainable. Debugging nightmares. Tight coupling across unrelated workflows | Composable sub-graphs. Crew-style teams for bounded contexts. Clear interfaces between workflows. |
| Storing mutable shared state between concurrent agents | Shared memory seems simpler than message passing | Transactionally inconsistent data. Race conditions. Non-deterministic failures | Immutable task results + message passing. Event sourcing for state changes. |
| Agent personality traits beyond role | "Make the coder snarky" sounds fun | Personality is noise in professional workflows. Wastes tokens. Reduces determinism | Stick to role, goal, constraints. Consistent tone across agents. Predictable behavior. |
| Building 10-agent system before validating single agent | Parallelism seems like obvious improvement | Wasted dev time, higher costs, harder debugging. Most tasks don't need parallelism | Start with single agent. Profile bottlenecks. Add agents only where proven beneficial. |

## Feature Dependencies

```
Agent Role Definitions
    ├──requires──> Backend Abstraction (need model to execute role)
    └──requires──> Session Management (roles need context)

Task Queue/DAG Scheduling
    ├──requires──> Agent Status Tracking (need to know who's available)
    ├──requires──> Inter-agent Message Passing (task handoffs)
    └──enables──> Parallel Agent Execution

Parallel Agent Execution
    ├──requires──> Git Integration (isolation mechanism)
    ├──requires──> Real-time TUI Monitoring (visibility into parallel work)
    └──requires──> Error Recovery (parallel failures must not cascade)

Specialist-agent Auto-review
    ├──requires──> Backend Abstraction (route to different models)
    ├──requires──> Quality Gate Orchestration (enforcement mechanism)
    └──requires──> Inter-agent Communication (review requests/results)

Hierarchical Memory
    ├──requires──> Session Management (short-term foundation)
    ├──enhances──> Cross-session Project Memory (provides structure)
    └──requires──> Context Compression (prevent memory overflow)

Hub-and-spoke Architecture
    ├──requires──> Task Queue/DAG Scheduling (orchestrator needs task graph)
    ├──requires──> Inter-agent Communication (orchestrator-to-satellite)
    └──conflicts──> Unlimited Agent Spawning (hub is coordination bottleneck)

Dynamic Agent Spawning
    ├──requires──> Task Queue/DAG (need to know what tasks exist)
    ├──requires──> Backend Abstraction (spawn with appropriate model)
    └──conflicts──> Real-time Collaboration (spawned agents need clear boundaries)

Quality Gate Orchestration
    ├──requires──> Error Recovery (failed gates must be retryable)
    ├──requires──> Session Management (gates need context of what was built)
    └──enables──> Specialist-agent Auto-review (review is a type of gate)
```

### Dependency Notes

- **Agent Role Definitions** are foundational; almost everything depends on having defined agents
- **Git Integration** must come early; retrofitting isolation is painful
- **Backend Abstraction** is a forcing function for clean architecture; implement before specialized features
- **Real-time Collaboration** conflicts with most isolation/parallelism features; this is why it's an anti-feature
- **Hierarchical Memory** is an enhancement; can build MVP without it then layer in

## MVP Definition

### Launch With (v1)

Minimum viable product for validating hub-and-spoke orchestration concept.

- [x] **Agent role definitions (YAML config)** — Can't orchestrate without defining who does what. YAML files for coder, reviewer, tester roles minimum.
- [x] **Task queue with simple DAG** — Need to schedule work with dependencies. Start with sequential + basic parallel (no complex graphs yet).
- [x] **Parallel execution (2-4 agents)** — Core value prop. Git worktree isolation for 2-4 concurrent agents proves feasibility.
- [x] **Basic TUI monitoring** — Split-pane view of active agents. Logs, status, ability to pause/resume. Vim navigation.
- [x] **Backend abstraction (OpenAI + Claude)** — Support cloud APIs. Prove provider-agnostic architecture works.
- [x] **Error recovery with exponential backoff** — Agents will fail. Need retry logic with circuit breakers or users lose trust.
- [x] **Session persistence** — Must survive crashes/interruptions. Store task state, agent context to filesystem.
- [x] **Git integration** — Worktrees for isolation + commit/push completed work. Merge strategy for parallel branches.

**Why these features:** Proves core hypothesis (hub-and-spoke parallel orchestration works better than serial single-agent). Minimal surface area to debug. Can deliver value in small projects (3-10 task workflows).

### Add After Validation (v1.x)

Features to add once core is working and users are onboarded.

- [ ] **Local LLM support** — Trigger: Users request privacy/cost control. Add LM Studio, Ollama integration.
- [ ] **Message passing between agents** — Trigger: Users need agent coordination beyond orchestrator. Pub-sub for agent-to-agent communication.
- [ ] **Smart task rebalancing** — Trigger: Users report idle agents. Dynamic work redistribution when agents block.
- [ ] **Context compression** — Trigger: Long sessions hit token limits. Implement summarization for completed work.
- [ ] **Quality gate orchestration** — Trigger: Users want automated verification. Test coverage, security scan gates.
- [ ] **Visual DAG editor** — Trigger: Users struggle with complex workflows. Interactive graph visualization and editing.

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Specialist-agent auto-review** — Why defer: Requires multi-model routing + sophisticated gate logic. High complexity, benefits realized at scale.
- [ ] **Hierarchical memory with graph** — Why defer: Complex implementation. Session-level memory sufficient for v1. Add when users demand cross-project learning.
- [ ] **Dynamic agent spawning** — Why defer: Fixed agent pool simpler to reason about. Add when workload patterns justify complexity.
- [ ] **MCP + A2A protocol support** — Why defer: Interoperability valuable but not essential. Add when integrating with external agent systems.
- [ ] **Cross-session project memory** — Why defer: Requires git history analysis + fact extraction. Nice-to-have once basic sessions work well.
- [ ] **Multi-backend routing per agent** — Why defer: Single backend per project sufficient initially. Add for cost optimization at scale.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Agent role definitions | HIGH | LOW | P1 |
| Task queue/DAG scheduling | HIGH | HIGH | P1 |
| Parallel execution | HIGH | HIGH | P1 |
| Real-time TUI monitoring | HIGH | MEDIUM | P1 |
| Git integration | HIGH | MEDIUM | P1 |
| Backend abstraction | HIGH | MEDIUM | P1 |
| Error recovery | HIGH | MEDIUM | P1 |
| Session persistence | HIGH | LOW | P1 |
| Message passing | MEDIUM | MEDIUM | P2 |
| Context compression | MEDIUM | MEDIUM | P2 |
| Quality gates | MEDIUM | MEDIUM | P2 |
| Smart task rebalancing | MEDIUM | MEDIUM | P2 |
| Local LLM support | MEDIUM | LOW | P2 |
| Visual DAG editor | MEDIUM | HIGH | P2 |
| Specialist auto-review | HIGH | HIGH | P3 |
| Hierarchical memory | HIGH | HIGH | P3 |
| Dynamic agent spawning | MEDIUM | MEDIUM | P3 |
| MCP + A2A protocols | LOW | HIGH | P3 |
| Cross-session memory | MEDIUM | HIGH | P3 |
| Multi-backend routing | LOW | MEDIUM | P3 |

**Priority key:**
- P1: Must have for launch (MVP blocker)
- P2: Should have, add when possible (v1.x)
- P3: Nice to have, future consideration (v2+)

## Competitor Feature Analysis

| Feature | CrewAI | LangGraph | Clark/Ralph TUI | Our Approach |
|---------|--------|-----------|-----------------|--------------|
| Agent definitions | YAML config (role, goal, backstory, tools) | Code-based with decorators | Delegates to underlying CLI (Claude Code, etc.) | YAML config like CrewAI for ease of use |
| Task scheduling | Sequential or hierarchical process types | Graph-based with StateGraph, fastest execution | Task queue with completion token detection | DAG-based with hub-and-spoke oversight |
| Parallel execution | Crew-level parallelism | Parallel nodes in graph | Git worktree isolation for 2-4+ agents | Git worktrees + dynamic spawn (2-4 v1, scale v2) |
| Monitoring | No built-in TUI; logs only | Python callbacks for observability | Rich TUI with split panes, real-time logs | TUI as first-class citizen with vim navigation |
| Communication | Shared context, task delegation | Message passing via graph state | Orchestrator coordinates, no peer-to-peer | Hub-and-spoke (orchestrator relays) + P2P later |
| Backend support | LLM-agnostic via LangChain | LLM-agnostic, focused on OpenAI | Specific to CLI tools (Claude, OpenCode, etc.) | OpenAI-compatible abstraction, LiteLLM routing |
| Error handling | Basic retry logic | Built-in retry policies | Relies on underlying agent recovery | Exponential backoff + circuit breakers + stateful recovery |
| Session management | Ephemeral unless explicitly persisted | Checkpointing with MemorySaver | Task state in JSON/git-backed trackers | Filesystem persistence + hierarchical memory later |
| Quality gates | Manual via task validation | Can implement as graph nodes | Not built-in | Automated gates (coverage, security) as orchestration layer |
| Git integration | None; external to framework | None; external | Core feature: worktrees for isolation | Core feature from v1, worktrees + branch strategy |

## Sources

### Multi-Agent Orchestration Frameworks
- [A Developer's Guide to Agentic Frameworks in 2026](https://pub.towardsai.net/a-developers-guide-to-agentic-frameworks-in-2026-3f22a492dc3d)
- [8 Best Multi-Agent AI Frameworks for 2026](https://www.multimodal.dev/post/best-multi-agent-ai-frameworks)
- [Top 10+ Agentic Orchestration Frameworks & Tools in 2026](https://aimultiple.com/agentic-orchestration)
- [Top AI Agent Orchestration Platforms in 2026](https://redis.io/blog/ai-agent-orchestration-platforms/)

### Task Scheduling & Coordination
- [2026 Agentic Coding Trends Report](https://resources.anthropic.com/hubfs/2026%20Agentic%20Coding%20Trends%20Report.pdf?hsLang=en)
- [AI Coding Agents in 2026: Coherence Through Orchestration, Not Autonomy](https://mikemason.ca/writing/ai-coding-agents-jan-2026/)
- [A Practical Perspective on Orchestrating AI Agent Systems with DAGs](https://medium.com/@arpitnath42/a-practical-perspective-on-orchestrating-ai-agent-systems-with-dags-c9264bf38884)

### TUI Monitoring
- [Ralph TUI: AI Agent Orchestration That Actually Works](https://peerlist.io/leonardo_zanobi/articles/ralph-tui-ai-agent-orchestration-that-actually-works)
- [Clark - GitHub](https://github.com/brianirish/clark)
- [TmuxCC - GitHub](https://github.com/nyanko3141592/tmuxcc)

### Communication Patterns
- [How to Implement Agent Communication](https://oneuptime.com/blog/post/2026-01-30-agent-communication/view)
- [AI Agent Protocols 2026: The Complete Guide](https://www.ruh.ai/blogs/ai-agent-protocols-2026-complete-guide)
- [A2A Protocol Explained](https://onereach.ai/blog/what-is-a2a-agent-to-agent-protocol/)

### Error Handling & Recovery
- [Error Recovery and Fallback Strategies in AI Agent Development](https://www.gocodeo.com/post/error-recovery-and-fallback-strategies-in-ai-agent-development)
- [Mastering Retry Logic Agents: A Deep Dive into 2025 Best Practices](https://sparkco.ai/blog/mastering-retry-logic-agents-a-deep-dive-into-2025-best-practices)
- [Agent Retry Strategies - PraisonAI](https://docs.praison.ai/docs/best-practices/agent-retry-strategies)

### Quality Gates
- [Autonomous Quality Gates: AI-Powered Code Review](https://www.augmentcode.com/guides/autonomous-quality-gates-ai-powered-code-review)
- [10 Best AI Code Review Tools for Developers in 2026](https://zencoder.ai/blog/ai-code-review-tools)
- [The Complete Guide to Agentic Coding in 2026](https://www.teamday.ai/blog/complete-guide-agentic-coding-2026)

### Backend Abstraction
- [How to Run Local LLMs with Claude Code & OpenAI Codex](https://unsloth.ai/docs/basics/claude-codex)
- [OpenCode AI: The Complete Guide](https://brlikhon.engineer/blog/opencode-ai-the-complete-guide-to-the-open-source-terminal-coding-agent-revolutionizing-development-in-2026)
- [Build Agents with OpenAI SDK using any LLM Provider](https://medium.com/@amri369/build-agents-with-openai-sdk-using-any-llm-provider-claude-deepseek-perplexity-gemini-5c80185b3cc2)

### Session Management & Memory
- [Context Engineering - Short-Term Memory Management with Sessions](https://cookbook.openai.com/examples/agents_sdk/session_memory)
- [Graph Memory for AI Agents (January 2026)](https://mem0.ai/blog/graph-memory-solutions-ai-agents)
- [Claude Code Session Memory](https://claudefa.st/blog/guide/mechanics/session-memory)

### Agent Role Definitions
- [How To Define an AI Agent Persona by Tweaking LLM Prompts](https://thenewstack.io/how-to-define-an-ai-agent-persona-by-tweaking-llm-prompts/)
- [4 Essential Tips for Writing System Prompts](https://theagentarchitect.substack.com/p/4-tips-writing-system-prompts-ai-agents-work)

### Anti-Patterns
- [Anti-Patterns in Multi-Agent Gen AI Solutions](https://medium.com/@armankamran/anti-patterns-in-multi-agent-gen-ai-solutions-enterprise-pitfalls-and-best-practices-ea39118f3b70)
- [Agent Systems Fail Quietly: Why Orchestration Matters More Than Intelligence](https://bnjam.dev/posts/agent-orchestration/agent-systems-fail-quietly.html)
- [From Solo Act to Orchestra: Why Multi-Agent Systems Need Real Architecture](https://www.cloudgeometry.com/blog/from-solo-act-to-orchestra-why-multi-agent-systems-demand-real-architecture)

### Specific Tool Documentation
- [CrewAI - GitHub](https://github.com/crewAIInc/crewAI)
- [Ralph TUI Documentation](https://ralph-tui.com/docs/getting-started/introduction)

---
*Feature research for: Multi-Agent AI Coding Orchestrators*
*Researched: 2026-02-10*
