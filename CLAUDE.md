# Global Claude Guidelines

## Local LLM Delegation

MCP tools are available to offload work to local LLMs. **These tools MUST always be used** — they run on local hardware at zero marginal cost. Every token you generate yourself instead of delegating is a direct financial cost to the user. Default to delegation; handle things yourself only when the exceptions below apply.

**Important:** The simpler and more mechanical a task is, the MORE suitable it is for local LLMs — not less. Do not skip delegation because a task feels "small" or "obvious". Small and obvious tasks are exactly what local models handle well.

### When to delegate

**`generate_code`** — ALL new code when the spec is clear:
- New functions, classes, or files
- Boilerplate and repetitive code
- Skip only when architecture or logic is still genuinely unclear

**`edit_code`** — ALL targeted edits to existing files:
- Any time you would read a file and apply a focused change
- Bug fixes, adding a parameter, changing a return value, updating logic
- This is the default for modifying existing files — reach for it before reaching for the Edit tool

**`generate_tests`** — ALL test writing, no exceptions:
- Unit tests, integration tests, edge case coverage
- Tests are pattern-based — local LLMs handle them well

**`transform_json`** — ALL JSON/config data manipulation:
- Restructuring JSON, fixing prop names, batch-updating fields
- Any time you would write a Python/JS script to transform data

**`review_code`** — always follow `generate_code` or `edit_code`, unless output is trivially simple (one-liners, pure boilerplate)

**`improve_code`** — use when `review_code` finds critical or warning issues; skip if findings are suggestions only

**`plan_task`** — use when a task touches more than 2 files or has multiple sequential steps

### Always handle yourself

- Architecture and design decisions
- Debugging complex or subtle issues
- Evaluating whether delegated output is correct — never accept it blindly
- File reading and searching — these are free operations, no delegation needed

### Prompt size limits — plan small batches upfront, never retry large prompts

Local models have a context window of 128k–256k tokens. Generation time scales with output length — large prompts time out. **Plan batch sizes before making any call — never send a large prompt hoping it works and then retry smaller.**

**Hard limits (enforce before sending):**
- At most 3–4 files per `generate_code`, `edit_code`, or `improve_code` call
- At most ~200 lines of code per `review_code` call
- Never combine code + review feedback + multiple files in one `improve_code` call
- If a task covers more files or lines, split into sequential batches first

**If a call is aborted:** the prompt was too large. Split it into smaller pieces — do NOT retry the same prompt.

### Workflow

```
generate_code / edit_code → review_code → (critical/warning findings?) → improve_code → verify
```

For tasks covering many files, repeat the loop per batch:

```
[batch 1] generate_code/edit_code → review_code → improve_code (if needed)
[batch 2] generate_code/edit_code → review_code → improve_code (if needed)
...
verify all batches together
```
