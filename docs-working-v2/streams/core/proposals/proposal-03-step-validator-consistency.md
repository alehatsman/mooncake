# Proposal 03: `mooncake step` enforces `additionalProperties: false` — close the validator gap

**Status:** Draft proposal
**Effort:** XS (~half day)
**Value:** Medium — closes a known asymmetry (finding #83). Tiny
refactor; significantly tightens the agent-facing entry point.

---

## Problem

Today `mooncake apply` and `mooncake step` validate inputs
differently:

```bash
# `apply` rejects unknown fields (post-MT-77 fix):
$ echo "- wait.command: { cmd: 'true', expected_exit: 0 }" > cfg.yml
$ mooncake apply -c cfg.yml
Error: cfg.yml
  Line 1: unknown field `expected_exit` ...

# `step` accepts the same unknown field silently:
$ mooncake step "wait.command: { cmd: 'true', expected_exit: 0 }"
{"changed": false, "action": "wait.command", ...}   # ran successfully
# But "expected_exit" is NOT a valid field — correct is "expect_exit"
# The action ran with the wrong expectation; no warning, no rejection.
```

**Receipt**: finding #83 from the 2026-05-15 audit. `step`
short-circuits the schema validator and routes directly into the
handler. Handlers only check their own required fields.

**Why it matters**: `mooncake step` is positioned as the
single-action MCP-style entry point ("Execute a single inline step
and return JSON result"). Agents call it. Agents make typos. The
agent's calling code doesn't know `expect_exit` from `expected_exit`
— that's what schema validation is for.

## Proposal

Route `mooncake step` through the same schema validator as
`mooncake apply` and `mooncake validate`:

1. Parse the YAML argument into a step structure
2. Run the validator (uses generated schema; rejects unknown fields)
3. Only then dispatch to the handler

```bash
$ mooncake step "wait.command: { cmd: 'true', expected_exit: 0 }"
Error: <inline>
  unknown field `expected_exit` (likely a typo or a renamed field — did you mean `expect_exit`?)
exit=1
```

(The "did you mean" hint comes from the closest-Levenshtein
suggestion already in MT-77.)

## API

No new flags. `mooncake step` becomes stricter, full stop.

Add `--no-strict` for back-compat only if needed:

```bash
mooncake step --no-strict "<old-shape>"
```

But the strict default is the right one — silently ignoring
unknown fields was the bug.

## Edge cases

- **Empty step** (`mooncake step ""`): error "step body is empty;
  expected an action".
- **Multiple actions** (`mooncake step "shell: ..., cmd: ..."`):
  error "step has multiple actions: shell, cmd" (same template as
  `apply`).
- **Action not in registry**: error "unknown action: X" + suggestions
  (same as `apply` MT-77 path).

## Receipts

From the audit:
- **#83** is the umbrella for this proposal.
- Throughout the audit I used `mooncake step` ~70 times for one-shot
  action tests. Several of those used typo'd field names and the
  command silently ran with the wrong arguments. Each cost ~30s of
  "wait, why didn't that work?" before I checked the schema.

For agents, the cost is worse: an agent that submits a typo'd
field gets back an action that ran, possibly partially. The agent
moves on assuming success. Future loop iterations reason from a
state that was actually never set.

## Why this lives in core

The validator is core's responsibility (`internal/config/`).
`mooncake step` lives in `cmd/` but its validation gate is a
shared library call. The fix is in the wiring between them, both
core-side.

## Implementation sketch

```go
// cmd/step.go
func runStep(yaml string) error {
    step, err := config.ParseStep(yaml)
    if err != nil { return err }

    // NEW: run the validator
    if err := config.ValidateStep(step); err != nil {
        return err   // same error shape as `mooncake apply` and `validate`
    }

    return executor.DispatchOne(step)
}
```

`config.ValidateStep` is a thin wrapper that calls the same
validator `mooncake validate` uses, scoped to one step.

## What this doesn't address

- **Schema completeness** — if the schema for action X is missing
  a property that the action actually accepts, the new validator
  rejects valid input. That's an SSOT-drift bug; same class as #27,
  fixed by ensuring `mooncake schema generate` is the source of
  truth.
- **Action-internal validation** — some handlers check
  cross-field consistency (e.g., file.copy requires either `src:`
  or `content:`, never both). That stays in the handler.

## Pairs with

- **DX proposal-04** (`mooncake actions show`) — exposes the
  valid-field list users can compare against
- **Proposal 01 (result schema)** — once strict, the
  `mooncake step` result is the canonical single-action shape
- **Agent proposal-01** (`describe_action` MCP tool) — same field
  inventory exposed via MCP
