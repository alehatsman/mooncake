---
id: F013
title: config.Step doc says "74 pointers" but actual count is 64; Creates/Unless are aliases that inflate the universal-field count
severity: doc
package: internal/config
file: internal/config/config.go
lines: 1309-1532
status: open
---

## What

Two adjacent issues in `type Step struct`.

### (a) "74 pointers" comment is stale

```go
// Actions (exactly one required — enforced at runtime by Validate()).
// The Go type system cannot express this as a compile-time union: all
// 74 pointers live on the struct and exactly one must be non-nil. Use
// DetermineActionType() to recover the action name; never switch on the
// fields directly.
```

(`config.go:1334`)

Actual count (auto-derived from the source):

```sh
awk '/^type Step struct/,/^}/' internal/config/config.go | grep -c 'action:'
# 64
```

The comment claims 74; the truth is 64. 10 action types have been
deleted or merged since the comment was written, but the comment
wasn't updated.

### (b) `Creates` / `Unless` are aliases for `UnlessExists` / `UnlessCommand`

```go
UnlessExists  *string `yaml:"unless_exists,omitempty" ...`
UnlessCommand *string `yaml:"unless_command,omitempty" ...`
Creates       *string `yaml:"creates,omitempty" ...`  // Alias: skip if path exists
Unless        *string `yaml:"unless,omitempty" ...`   // Alias: skip if command succeeds
```

(`config.go:1317-1328`)

Two pairs of fields express two skip-conditions. The comment
(line 1320-1326) explains: aliases exist so users don't have to
relearn the verb across step-level vs action-level.

These two aliases account for **2 of the 36 universal fields**.
At 36/40 the cap is "within 20%". If the project later decides to
deprecate the long forms (`unless_exists` / `unless_command`) and
keep only the friendly aliases (`creates` / `unless`), the count
drops to 34. Conversely if the friendly aliases are eventually
the deprecated ones, the count also drops.

That's a real-payoff knob the soft-cap policy reviewer should be
aware of: **the next two universal-field deletions that push the
count under 35 are already-known duplicate aliases, not new work.**

## Why it's `doc` not `risk`

- (a) is a stale comment. Will mislead the next reviewer.
- (b) is a documented choice with a real ergonomics rationale.
  Recording it here so the reviewer who reaches for "shrink the
  Step struct" has the context.

## Suggested fix

For (a):

Replace the hardcoded count with a sentence pointing at the budget
script:

```go
// Actions (exactly one required — enforced at runtime by Validate()).
// The Go type system cannot express this as a compile-time union:
// all action pointers live on the struct (run `make budget-status`
// for the current count) and exactly one must be non-nil. Use
// DetermineActionType() to recover the action name; never switch
// on the fields directly.
```

This matches the policy direction already taken for the soft-cap
list in CLAUDE.md (F002).

For (b):

No code change. Update the field comment to record that the count
includes redundant aliases and link to F013 so a future field-shrink
PR knows where to look:

```go
// Idempotency controls (4 fields total; UnlessExists/UnlessCommand
// are the canonical names and Creates/Unless are friendly aliases —
// see F013 in docs-working/code-review for the policy rationale).
UnlessExists  *string ...
UnlessCommand *string ...
Creates       *string ...
Unless        *string ...
```

## Verification

- Re-run `awk` count above; comment matches.
- `make budget-status` — confirm 36 universal fields is the
  authoritative number.
- Grep for `// all 74` etc. in case the number leaks elsewhere.

## References

- F002 — same documentation-drift pattern in CLAUDE.md.
- `scripts/budget-status.sh:67-83` — the universal-field counter.
