# Spec 30: `transaction:` Blocks — Reverse-on-Failure

**Status:** Draft (depends on spec 22)
**Epic:** E9 Modern Action Surface — bucket E9.4
**Effort:** L (1–2 weeks)
**Value:** Killer demo. Headline feature for the "Mooncake as agent
safety substrate" pitch (see `VISION.md` §7). A `transaction:` that
auto-rolls-back makes "let the agent try" actually safe.

**Design principles:** `docs-working/action-design-principles.md`

**Depends on:** spec 22 (extended Handler ABI — specifically `Reverse`).

---

## Problem

Two scenarios, same fundamental need:

1. **Multi-step provisioning.** Install postgres, create a user,
   create a database. If step 3 fails (e.g. disk full), steps 1+2
   have already taken effect. The user is left with a half-set-up
   system that nobody owns: roll forward is hard, manual rollback
   is fiddly.
2. **Agent tries something risky.** An LLM agent writes a Mooncake
   plan that does several things in sequence. Mid-plan, one step
   fails. We want all-or-nothing semantics so the agent doesn't
   leave inconsistent state.

`try`/`catch`/`finally` (spec 23) lets users *write* rollback logic
by hand. `transaction:` automates it via the `Reverse` ABI (spec 22).

---

## Goals

- **G1** Add `transaction:` keyword on Step. Value is a sequence of
  Steps that all apply or all roll back.
- **G2** On any step's failure inside a transaction, previously-
  applied steps' `Reverse()` runs in LIFO order.
- **G3** Surface in plan output as a single transaction node with
  child steps and explicit `reversible: true|false` per child.
- **G4** Refuse to plan a transaction containing a step whose
  Reverser is missing or declares the step irreversible — *unless*
  the user opts in with `allow_irreversible: true`.
- **G5** Roll-forward semantics for partial-success state: if any
  Reverse fails mid-rollback, halt and surface a clear "manual
  intervention required" error with the exact state.

**Out of scope:**

- Distributed transactions across hosts (Mooncake daemon territory).
- Saga-style compensating actions (different shape from `Reverse`).
- Snapshot-and-restore at the filesystem level (way too heavy).

---

## Design

### YAML shape

```yaml
- transaction:
    - pkg.install: { name: postgresql }
    - os.user: { name: app }
    - shell: createdb -U postgres app
    - file.template: { src: app.conf.j2, dest: /etc/app/app.conf }
  on_rollback:
    - log: "transaction rolled back; manual cleanup may be needed if rollback itself errored"
  allow_irreversible: false
```

Semantics:

1. **Plan phase**: each child step's `Reverse()` is queried (returns
   the inverse Step or nil-with-error if irreversible).
   - If any child is irreversible AND `allow_irreversible: false`:
     plan fails with a clear error naming the step.
   - Otherwise plan succeeds; transaction node includes the inverse
     Step list (as plan metadata).
2. **Apply phase**: children run sequentially.
   - On success: transaction commits (no rollback). Outputs of children
     are exposed under `outputs.children[0..N]`.
   - On failure of step K: applied children (0..K-1) are reversed in
     LIFO order. K's partial state is also passed to its Reverse if
     the handler reports a partial-apply result.
   - If a Reverse itself fails: halt rollback at that point, surface
     a `RollbackFailedError` listing what's been reverted and what
     hasn't. The on_rollback block (if defined) still runs.
3. **`on_rollback:`** is sugar — same shape as `on_change:` but fires
   on rollback. Useful for notifications.

### Step struct

```go
type Step struct {
    // ...
    Transaction        []Step `yaml:"transaction" json:"transaction,omitempty"`
    OnRollback         []Step `yaml:"on_rollback" json:"on_rollback,omitempty"`
    AllowIrreversible  bool   `yaml:"allow_irreversible" json:"allow_irreversible,omitempty"`
}
```

Validation: if `Transaction` is set, no other action field; `OnRollback`
must be empty if `Transaction` is empty.

### Plan output

`mooncake plan --format json` emits:

```json
{
  "id": "txn-deploy",
  "kind": "transaction",
  "children": [
    { "step": "pkg.install postgresql", "reversible": true, "reverse_via": "pkg.install state=absent" },
    { "step": "os.user app",            "reversible": true, "reverse_via": "os.user state=absent" },
    ...
  ]
}
```

### Failure UX

On rollback, run output looks like:

```
✗  Step 3 of 4 failed: createdb -U postgres app
↺  Rolling back...
↺  Reversed step 2: os.user app (removed)
↺  Reversed step 1: pkg.install postgresql (removed)
✗  Transaction failed; system reverted to pre-transaction state.
```

If rollback partially fails:

```
✗  Step 3 of 4 failed.
↺  Rolling back...
↺  Reversed step 2: os.user app (removed)
✗✗ Failed to reverse step 1: pkg.install postgresql (apt error)
‼  ROLLBACK INCOMPLETE — system in indeterminate state.
   Successfully reverted: [step 2]
   Failed to revert:      [step 1]
   Manual intervention required.
```

---

## Key files

| File | Change |
|---|---|
| `internal/config/config.go` | Step.Transaction + OnRollback + AllowIrreversible fields. |
| `internal/plan/planner.go` | Expand transaction blocks. Query each child's Reverser at plan time. |
| `internal/executor/executor.go` | Compound execution: apply forward, reverse on failure (LIFO). |
| `internal/executor/transaction.go` | New file. Transaction state machine. |
| `internal/runlog/runlog.go` | New event types: `transaction_begin`, `transaction_commit`, `transaction_rollback_begin`, `transaction_step_reversed`, `transaction_rollback_complete`, `transaction_rollback_failed`. |
| `internal/config/schema.json` etc. | Regenerate. |
| Examples | `examples/transactions/postgres-bootstrap.yml` |

---

## Tasks (phased)

1. **Phase 1** — Step struct + parser + schema. No execution semantics yet.
2. **Phase 2** — Plan-phase reversibility check. Reject irreversible
   transactions unless allow_irreversible.
3. **Phase 3** — Forward apply with success commit.
4. **Phase 4** — Rollback on failure (LIFO). Tests with handler-level
   mocks (fail step K, assert steps 0..K-1 reversed).
5. **Phase 5** — Partial-rollback error path.
6. **Phase 6** — on_rollback sugar.
7. **Phase 7** — Run output formatting + JSON plan output.
8. **Phase 8** — Docs + examples.

---

## Acceptance criteria

- Transaction of 4 steps where step 3 fails: steps 2 and 1 reversed;
  exit code non-zero; final system state byte-identical to
  pre-transaction.
- Transaction containing `shell:` (irreversible) fails at plan time
  with a clear error unless `allow_irreversible: true`.
- Partial rollback failure surfaces `‼ ROLLBACK INCOMPLETE` with
  exact step list.
- `mooncake plan --format json` of a transaction shows children +
  reverse_via per child.
- Build / vet / lint / test green.

---

## Open questions

1. **Concurrency inside a transaction** — should children run
   sequentially only, or allow `parallel:` for independent ones?
   Lean: sequential only. Concurrency is a follow-up.
2. **What about transactions across reboots?** Out of scope — if a
   step requires reboot, it's irreversible by definition for this
   spec.
3. **Should `transaction:` nest?** Lean: yes, naturally. Nested
   transactions roll back outward.
4. **`allow_irreversible: true` — per-step or transaction-wide?**
   Probably transaction-wide for v1; per-step granularity is a
   follow-up if needed.
5. **What's the semantic of `on_change`/`try` inside a transaction?**
   Probably allowed; they're just shapes of children. Their own
   error semantics still apply. Codify in tests.
