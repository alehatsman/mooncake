# Proposal 06: Reconcile `failed: false` + `error: "..."` — query-style vs. error-style outcomes

**Status:** SHIPPED (2026-05-28, worktree-result-envelope) — bundled
with proposals 01 + 02. Two-pronged fix:
  - Central envelope sync in `dispatchRunner` (executor.go ~L1482):
    any handler returning `(result, err)` with err != nil now lands
    on the envelope as Failed=true + Error=err.Error(). Catches the
    wait.* / os.mount / os.firewall / pkg.* / container cluster
    without touching each handler — spec-69-followups finding B0
    closed at the same time.
  - observe.* migration to a new `result.PublishObservation(env,
    target)` helper that drops env.Error into Data["error"] (the
    old shape) and instead promotes probe-side env.Error to
    envelope Error + Failed. Plan-mode "deferred" message now
    rides on Reason, not on a magic env.Error sentinel.
**Effort:** S (~2 days; handler-by-handler cleanup) — actual:
folded into the proposal-01 sweep.
**Value:** Medium — recurring confusion across observe.* / wait.* /
os.* actions. Fixing the convention removes a class of agent-side
"is this really a failure?" bugs.

---

## Problem

Several actions today return `failed: false` while populating
`error:` with a real diagnostic. From the 2026-05-15 audit (#61):

```
observe.process (process not running):
  {"failed": false, "found": false, "error": "no matching process"}

wait.http (timeout reached):
  {"failed": false, "success": false, "error": "wait.http timeout after 1s"}

os.mount (mount failure):
  {"failed": false, "error": "os.mount: mount /tmp/tmpfs: exit status 32"}

os.firewall (iptables permission denied):
  {"failed": false, "error": "os.firewall: ... iptables: Permission denied"}
```

The callers (agents, scripts, humans) face three different meanings
glued onto the same shape:

| Action | What `failed: false, error: <X>` means |
|---|---|
| `observe.process` | The query completed; the answer is "absent". Not an error. |
| `wait.http` | The timeout fired; the wait conclusion is "didn't reach desired state". Could be an error OR a deliberate "did the service come up?" probe. |
| `os.mount` | The mount **actually failed**. State on host is wrong. This is an error. |
| `os.firewall` | Same — the change didn't happen. Error. |

Agents that check `failed` see false → success. Reality:
two of four are real failures.

## Proposal

A clear three-way taxonomy:

1. **Query-style action** (observe.*, wait.*, read.*, repo.search/tree):
   - Returns `operation: "query"` (per proposal 01)
   - "Empty result" / "absent" / "timeout" is NOT failure
   - `failed: false`, `error: ""`, `value.found: false` (or equivalent typed field)
   - Convention: **query that returns "no" is success**

2. **Mutation-style action** (file.*, pkg.*, os.user, os.firewall, ...):
   - Returns `operation: create|update|delete|noop|reverted`
   - If the state on disk/host wasn't reached, `failed: true`, `error: "<reason>"`
   - Convention: **mutation that didn't happen is failure**

3. **Mixed-mode action** (`assert`):
   - Operation = `query` but with explicit success/fail semantics
   - assert passes → `failed: false`
   - assert fails → `failed: true, error: "<assertion message>"`
   - Convention: **assertions are explicit about whether the answer
     is acceptable**

### Per-action transformations

| Today | Tomorrow |
|---|---|
| `observe.process` not running: `failed: false, error: "no matching process"` | `failed: false, error: "", value: {running: false}` |
| `wait.http` timeout: `failed: false, success: false, error: "...timeout..."` | `failed: true, error: "wait.http timeout after 1s..."` ¹ |
| `os.mount` mount-failed: `failed: false, error: "mount failed"` | `failed: true, error: "mount failed: ..."` |
| `os.firewall` perm-denied: `failed: false, error: "Permission denied"` | `failed: true, error: "permission denied: ..."` |

¹ Open question: is `wait.http` query or mutation? It "waits" for
a condition. Argument: **timeout IS a failure** because the
desired condition (HTTP ready) wasn't reached within the budget.
The user opted into `wait` expecting it to succeed if the
condition holds. Recommend treating wait.* as **mutation-style**:
timeout = `failed: true`.

Counter-argument: in some workflows, `wait.http --timeout 5s` is
a "probe; carry on if it didn't come up" — the agent checks the
outcome via `success: false` and decides. Recommend: keep
`success:` field on wait.* (already there) AND set `failed:
true` on timeout. Agents choose which to consume.

## Field standardization

Three fields, three meanings:

- `failed: bool` — "was this action a failure that should affect
  the recap?" (used by counter)
- `error: string` — "if there was a diagnostic to surface, here it
  is" — empty string when no error
- `value: {found: bool, ...}` — for query-style: whether the
  thing being observed exists / matches

Rule of thumb: **`error: ""` when `failed: false`**. The two are
linked. Population of `error` without `failed: true` is the bug
this proposal fixes.

### `assert` is the exception

`assert` is the one place where `error: "<message>"` with `failed:
true` is correct AND the error message *is* the answer (assertion
failed because: X). Document this as the canonical "error reflects
the assertion's no-answer".

## Impact on recap counters

This proposal cleanly feeds proposal-02:

- `failed: true` → counted in `failed=N` in recap
- `failed: false, error: ""` → counted in `ok` or `changed`
  depending on `changed:` field

Today's confusion ("did wait.http fail? recap says failed=1 via
apply, but step result says failed:false") goes away.

## Receipts

From the audit (#61 cited each of these):
- `observe.process` with not-running name
- `wait.http` timeout
- `os.mount` ENOTDIR / EACCES
- `os.firewall` iptables permission denied
- `container` (proposal-related to #64): `.ImageName` template error
  with `failed: false`

Each became its own ad-hoc finding because the calling code didn't
know whether to trust `failed`. Codifying the convention removes the
class.

## Implementation pattern

Each handler's `Execute()` builds a `Result`. Add a single helper:

```go
// internal/actions/result/builder.go
func QueryResult(value map[string]any) *Result {
    return &Result{
        Operation: OpQuery,
        Failed:    false,
        Error:     "",
        Value:     value,
        Changed:   false,
    }
}

func MutationFailedResult(err error, partialValue map[string]any) *Result {
    return &Result{
        Operation: OpUpdate,
        Failed:    true,
        Error:     err.Error(),
        Value:     partialValue,
        Changed:   false,
    }
}
```

Migrate handlers handler-by-handler. The audit findings list is the
priority queue.

## Why this lives in core

Result shape and `failed` semantics are core's contract. Every
consumer (text formatter, MCP server, agentd, JSON channel, fleet
exec multiplexer) decides how to render based on these fields. One
discipline, many beneficiaries.

## Pairs with

- **Proposal 01 (result schema)** — `Operation` enum + `Failed`
  field are the inputs
- **Proposal 02 (recap counter)** — uses `Failed` for the `failed=N`
  bucket
- **Agent proposal-06 (permissions as contract)** — agents need
  to trust `failed` to gate the next step in a loop
