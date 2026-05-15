# Proposal 05: Surface action capability flags — make `Permissions/Diff/Cost/Reverse` visible

**Status:** Draft proposal
**Effort:** XS (~1 day; mostly registry plumbing)
**Value:** Medium — the four-method handler ABI (spec-22) is core's
big bet, but its outputs aren't easily inspectable. Exposing them
lets users and agents know which actions are "fully typed" vs.
which still have refusal stubs.

---

## Problem

Spec-22 defines four ABI methods per handler:
- `Permissions()` — what does this action touch?
- `Diff()` — what would change?
- `Cost()` — how risky/expensive?
- `Reverse()` — how to undo?

Some handlers implement all four; some implement subsets. Today
`mooncake actions list` shows a table with `SUDO` and `CHECK`
columns, but the four ABI capabilities aren't visible:

```
ACTION          CATEGORY   PLATFORMS                 SUDO     CHECK
artifact.capture system     all                       no       no
...
```

Asking "is `git.checkout` reversible?" requires reading source.
Asking "does `pkg` implement `Cost`?" requires reading source.
Asking "which actions return refusal stubs?" requires reading
source.

This matters because:
1. Users planning destructive operations want to know "can this
   be reversed?" before applying.
2. Agents writing `transaction:` blocks must check that every
   child action implements `Reverse` — otherwise rollback is a
   no-op. Today this is implicit; should be a hard error at plan
   time (and a visible warning in `actions show`).
3. New handler authors need a checklist; today the four methods
   are scattered across handler files.

## Proposal

Add four columns to the action inventory:

```
$ mooncake actions list
ACTION          CATEGORY   PLATFORMS            SUDO  CHECK  DIFF  COST  REVERSE
file.write      file       all                   no   yes    yes   yes   yes
file.copy       file       all                   no   yes    yes   yes   yes
pkg             system     linux,darwin,windows  yes  yes    yes   yes   partial
os.service      system     linux,darwin,windows  yes  yes    yes   yes   no
git.checkout    system     all                   no   yes    yes   yes   yes
git.config      system     all                   no   yes    yes   yes   yes
text.replace    file       all                   no   yes    yes   yes   yes
shell           command    all                   yes  no     no    no    no
```

Values: `yes` / `no` / `partial` (implements method but returns
informative stub for some inputs).

Also expose in the schema (per `mooncake schema generate`):

```json
"file.write": {
  "type": "object",
  "x-implements-check": true,
  "x-implements-diff": true,
  "x-implements-cost": true,
  "x-implements-reverse": true,
  "x-reverse-strategy": "delete-on-rollback",
  ...
}
```

`mooncake actions show <name>` (per DX proposal-04) gets a
"Capabilities" section:

```
$ mooncake actions show pkg

pkg
───
Install/remove packages via the OS package manager.

Capabilities:
  Permissions:  filesystem (system paths), network (download)
  Check:        yes — compares declared vs. installed
  Diff:         yes — shows version transition
  Cost:         medium — install can pull dependencies + 100+ MB
  Reverse:      partial — uninstalls explicitly-installed packages, NOT dependencies pulled in
                Rationale: dependency tree may now be load-bearing for other packages.

Platforms:    linux, darwin, windows (apt, dnf, pacman, brew, choco)
Requires:     sudo on Linux for system-wide installs
```

For agents:

```bash
$ mooncake actions show pkg --format json
{
  ...,
  "x-implements-reverse": "partial",
  "x-reverse-notes": "uninstalls explicitly-installed packages, NOT dependencies"
}
```

## Plan-time enforcement

When `mooncake plan` (or `apply`) encounters a `transaction:` with
a child whose `Reverse() = no`, today the run proceeds. spec-30
shipped LIFO rollback but if any child can't reverse, the
transaction's atomicity is a lie.

Proposed plan-time check:

```
$ mooncake plan -c cfg.yml
Plan: cfg.yml
↑ transaction (3 children)
  ↑ child 1: file.write /etc/foo.conf   (reversible)
  ↑ child 2: os.service nginx enable    ⚠ NOT REVERSIBLE
  ↑ child 3: pkg install nginx          (partial: dep tree not reversible)

  ✗ Plan-time error: transaction child 2 (os.service) does not
    implement Reverse(). Set `allow_irreversible: true` to proceed
    OR replace the child with a reversible action.
```

The plan-time error stops irreversible-in-a-transaction from
shipping. Maps directly to the existing #67 fix that rejected
nested-try with an "allow_irreversible" hint.

## Capabilities as part of the contract

This proposal makes capabilities first-class:
- Visible in `actions list` (the inventory)
- Visible in `actions show` (per-action details)
- Visible in the schema (machine-readable)
- Enforced at plan time (transactions can't accept irreversible
  children without opt-in)

Each is a small change; together they make the four-method ABI
visible to everyone consuming the action surface.

## Receipts

From the audit:
- `mooncake actions list` already has a SUDO and CHECK column —
  proves the pattern. Add Diff/Cost/Reverse to match.
- Per-action reversibility came up implicitly in spec-26's
  reverse-capture work. The capability matrix would summarize
  spec-26's progress at a glance.
- Several handlers have refusal stubs (per the dx/README's "Open
  gaps" mention of "spec-26 reverse-capture rollout to refusing
  handlers"). A `Reverse: no` column makes those visible.

## Implementation sketch

The registry already tracks capability metadata internally — each
handler embeds an `ActionMetadata` struct. Add explicit
`ImplementsXxx` bools to that struct + a method:

```go
type ActionMetadata struct {
    Name string
    Category string
    // ...
    ImplementsCheck   Capability  // yes/no/partial
    ImplementsDiff    Capability
    ImplementsCost    Capability
    ImplementsReverse Capability
    ReverseStrategy   string  // optional rationale string
}

type Capability string
const (
    CapYes     Capability = "yes"
    CapNo      Capability = "no"
    CapPartial Capability = "partial"
)
```

The registry is already the source of truth for `actions list`;
extend that pipeline.

Plan-time check: in `internal/plan/transaction.go`, after expansion,
walk each transaction's children and check
`child.metadata.ImplementsReverse != CapYes`. Error if any.

## What this doesn't address

- **Forward-compat schema** for actions that gain capabilities
  over time. If `os.service.Reverse` ships in v0.4, every existing
  test playbook that asserted "irreversible" against it has to
  adapt. Mitigation: capability changes are minor-version bumps,
  not patch.
- **Runtime fallback**. If `Reverse()` is implemented but errors at
  runtime (e.g., the dependency tree is unrecoverable), the
  transaction still fails. That's a runtime concern, not a
  declaration concern; logged separately.

## Pairs with

- **DX proposal-04** (`mooncake actions show`) — the place users
  see the capability matrix per action
- **Proposal 01 (result schema)** — `Reverse()` for committed
  steps emits `operation: reverted`
- **spec-22** (extended handler ABI) — this proposal makes spec-22's
  outputs visible
- **spec-26** (reverse-capture rollout) — the column tells operators
  which handlers still need work
