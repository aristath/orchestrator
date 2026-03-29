# Global Claude Guidelines

## Local LLM Delegation

MCP tools are available to offload work to local LLMs. Use them to reduce token usage on tasks that don't require higher reasoning.

### When to delegate

**`generate_code`** — prefer this for first drafts when the spec is clear:
- New functions, classes, or files
- Boilerplate and repetitive code (tests, adapters, converters)
- Skip when the architecture or logic is still unclear — draft it yourself in that case

**`review_code`** — always follow `generate_code` with this, unless the output is trivially simple (one-liners, pure boilerplate)

**`improve_code`** — use when `review_code` finds critical or warning issues; skip if findings are suggestions only

**`plan_task`** — use when a task touches more than 2 files or has multiple sequential steps

### Always handle yourself

- Architecture and design decisions
- Debugging complex or subtle issues
- Evaluating whether delegated output is correct — never accept it blindly
- File reading and searching — these are free operations, no delegation needed

### Workflow

```
generate_code → review_code → (critical/warning findings?) → improve_code → verify
```
