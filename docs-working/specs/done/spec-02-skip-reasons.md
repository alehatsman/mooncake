# Spec 02: Skip Reasons

## Problem

When a `when:` condition is false, or `creates:` path exists, the step silently
disappears from output. On a 79-step run this makes it impossible to understand
why half the steps did nothing.

Current `renderStepSkipped` exists but the skip reason is just `"when"`,
`"tags"`, or `"idempotency:<path>"` — not the actual expression or value.

## Goal

Every skipped step prints one line with the reason:

```
─  Install OpenJDK (Debian/Ubuntu)          [when: apt_available]
─  Install Python build deps (Ubuntu)       [when: apt_available]
─  Install pyenv                            [creates: ~/.pyenv/bin/pyenv]
─  Install rustup (Linux/macOS)             [when: rustup_installed.rc != 0]
─  Install OpenJDK (Arch Linux)             [tags]
```

## Behavior

- `─` prefix (dash, distinct from `✓` ok and `✗` fail)
- Step name left-aligned, reason right in brackets
- Reason content:
  - `when: <expression>` — show the actual expression from the step, not just "when"
  - `creates: <path>` — show the expanded path
  - `unless: <cmd>` — show the command
  - `tags` — just "tags" (no expression needed)
- Steps skipped by tags: only show if `--verbose` flag set (tag skips are
  usually uninteresting noise); all other skip reasons always shown
- Indented same as regular steps (level-aware)

## Implementation

`StepSkippedData.Reason` currently holds `"when"`, `"tags"`,
`"idempotency:<path>"`. Need to enrich it to include the actual expression.

In `executor.go`:
- `HandleWhenExpression`: pass rendered `whenExpression` string into skip reason
  → `"when: os == \"linux\" and apt_available"`
- `CheckIdempotencyConditions`: already returns the path in reason
  → format as `"creates: /home/user/.pyenv/bin/pyenv"`
- `ShouldSkipByTags`: reason stays `"tags"`

`StepSkippedData.Reason` field is a string — keep it, just make it richer.

`renderStepSkipped` in `console_subscriber.go` — update to render with `─` prefix
and bracket formatting.

## Out of scope

- Showing the evaluated value of the expression (e.g. `apt_available = false`)
- Aggregating skips into a summary block
- Skip reasons in agent JSONL (that comes naturally from the existing Reason field)
