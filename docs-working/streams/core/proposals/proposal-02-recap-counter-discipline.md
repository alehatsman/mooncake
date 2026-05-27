# Proposal 02: Recap counter discipline — `ok / changed / skipped / failed / reverted / cancelled`

**Status:** SHIPPED (2026-05-28, worktree-result-envelope) — bundled
with proposals 01 + 06. `cancelled` counter is in
events.RunCompletedData + ExecutionStats + the recap renderer;
the macro-level SIGINT-loop between-step bump is in place. Per-
step "in-flight at cancel" attribution is a follow-up — the loop
sees the cancel between steps, not from the in-flight handler.
Exit code rules (failed>0 → 1, cancelled>0 → 130) are honored at
the existing CLI signal-handler layer in cmd/kernel/apply.go.
**Effort:** S (~2 days; mostly executor + renderer)
**Value:** High — the recap is the user's eyeball summary AND CI's
exit signal. Multiple findings traced to counter ambiguity. A
crisp taxonomy is the simplest fix.

---

## Problem

The recap line:
```
RECAP  ok=N  changed=N  skipped=N  failed=N  duration  hint
```

…is the headline output of every `mooncake apply`. But the bucket
definitions aren't crisp:

| Scenario | Today |
|---|---|
| `file.write` to identical content | `ok` (no change) ✓ |
| `shell` runs successfully | `changed` (everything is "changed" by default) |
| `shell` with `creates:` guard met | was `changed` (#2 bug); now `skipped` ✓ |
| step with `when: false` | `skipped` |
| step that succeeded inside transaction, then transaction failed | was `changed`; now `reverted` (#45 fix) |
| step that failed but `failed_when: false` | `ok` (failure suppressed) — confusing |
| step inside `try:` that failed, `catch:` handled it | step itself shows `failed`, recap counts it as `failed=1` even though run "succeeded" overall |
| step cancelled mid-execution (proposal fleet-02) | undefined — no counter |
| run interrupted by SIGINT (#87) | undefined — no entry in `runs.jsonl` |

Receipts:
- **#2** — `creates:`/`unless:` shell guards counted as changed
- **#45** — transactions miscounted reverted as changed
- **#28** — `failed_when: false` on assert didn't update the recap
- **#48** — `retry: + failed_when: false` interaction was confusing
- **#87** — SIGINT mid-run has no recap representation

## Proposal

Codify **six recap counters** with sharp definitions:

| Counter | Meaning | Example |
|---|---|---|
| `ok` | Step ran AND made no change. State was already at target. | `file.write` to identical content; `pkg` already installed |
| `changed` | Step ran AND mutated state on disk. | `file.write` content differs; new package installed |
| `skipped` | Step did NOT run. Reason in `result.skipped_reason`. | `when: false`, `creates:` met, `on_change` parent didn't change |
| `failed` | Step ran AND returned `failed: true`. NOT masked by `failed_when:`. | assert failed, command exited non-zero |
| `reverted` | Step ran, then was rolled back by a parent transaction's LIFO Reverse(). | step-0001/0002 in `examples/transactions/rollback-demo.yml` |
| `cancelled` | Step was in-flight when the run was cancelled / interrupted. | SIGINT mid-run; `fleet kill` |

Plus a derived "exit shape" — how this affects `mooncake apply`'s
process exit code:

| Recap state | Exit | Meaning |
|---|---|---|
| `failed=0 AND cancelled=0` | 0 | Clean run |
| `failed>0` | 1 | One or more steps failed |
| `cancelled>0 AND failed=0` | 130 (SIGINT-equivalent) | User cancelled |
| Parse error / planner setup | 2 | Couldn't even start |

The recap line itself expands:

```
RECAP  ok=2  changed=5  skipped=1  failed=0  reverted=0  cancelled=0  1.2s
```

Or, in compact mode, suppress zero counters:

```
RECAP  ok=2  changed=5  skipped=1  1.2s
```

(Already half-implemented; codify the rule.)

## How each scenario lands

| Scenario | New counter | Why |
|---|---|---|
| `file.write` identical content | `ok` | Made no change |
| `shell: echo hi` | `changed` | Side-effect made (stdout produced) — but see open question |
| `shell` with `creates:` guard met | `skipped` (reason: creates) | Didn't run |
| `when: false` | `skipped` (reason: when) | Didn't run |
| `on_change:` parent unchanged | `skipped` (reason: on_change) | Didn't run |
| `failed_when: false` on a failing assert | `ok` | failure suppressed → success |
| try{X fail} catch{Y ok} finally{Z ok} | `failed=1, ok=2` | Each step counts independently. Run exit determined by aggregator (see below) |
| transaction with rollback | `reverted=N, failed=1` | LIFO reverse path |
| SIGINT mid-run | `cancelled=N, failed=0` (where N = in-flight step) | Cancellation isn't failure |

### Open question: `shell: echo hi` — `ok` or `changed`?

Two camps:
1. **Camp "shell is always changed unless guarded"**: bash without
   `changed_when: ...` is opaque; assume the worst.
2. **Camp "shell with rc=0 and no stdout/stderr is ok"**: shell that
   "looks pure" is `ok`; explicit `changed_when:` overrides.

Today's behavior is camp 1 (every shell is `changed`). Camp 2 is
more honest but harder (no good heuristic for "shell action with
side effects").

**Recommend keeping camp 1** (status quo) but documenting it.
Users who want fine-grained signal use `changed_when:` or the
typed actions.

## Exit code aggregation

`run.completed` event already has `success: bool`. Make the rule
explicit:

```python
def run_exit_code(counters: dict) -> int:
    if counters['failed'] > 0:
        return 1
    if counters['cancelled'] > 0:
        return 130  # standard SIGINT exit
    return 0
```

`try/catch/finally`'s "caught" failure: today the run exits non-zero
even when catch handles it. **Recommend keeping this** (a caught
failure is still a failure for audit purposes) but document — see
issue #21 era.

## API impact

- New: `result.skipped_reason` enum field. Values: `when`, `creates`,
  `unless`, `on_change`, `tag_filter`, `try_already_failed`.
- New: `result.cancelled_reason` enum field. Values: `sigint`,
  `fleet_kill`, `timeout`.
- New: text formatter line markers — `- step` (skipped) stays;
  add `↺ step` for reverted (already exists in transactions),
  `⊘ step` for cancelled.

## Receipts

The fixes that landed already pre-figured this proposal:
- **#2 / MT-2** — shell creates/unless → skipped
- **#45** — transaction reverted shows in recap
- **#28** — failed_when honored on assert
- **#48** — retry + failed_when interaction

Codifying the taxonomy makes the next refactor (e.g., adding
`cancelled` per fleet proposal-02) drop in cleanly instead of
inventing yet-another-bucket.

## What this doesn't address

- **Granular per-step skipped reasons in the text output** — the
  enum exists in `result`; rendering it in the compact recap is
  optional. Default: show in the per-step line (`- write [when:
  os == "darwin"]`), not the recap.
- **Custom counter buckets** — agents that want to count "files
  touched" or "packages installed" separately should compute
  from `result.operation` (proposal-01) rather than expand recap.

## Pairs with

- **Proposal 01** (result schema) — `Operation` enum + `Failed`
  bool are the inputs to the counter math.
- **Proposal 06** (failed/error distinction) — the question "what
  counts as failed" is settled there.
- **Fleet proposal-02** (fleet kill) — adds `cancelled` counter
  use case.
