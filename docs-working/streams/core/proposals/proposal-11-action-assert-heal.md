# Proposal 11: `assert` action + on-fail `heal:` handler — self-healing as a primitive

**Status:** Partially shipped — the `assert` action exists at `internal/actions/assert/` (multiple sub-fields: command, file_exists, file_sha256, git-clean / git-diff). **Open:** the on-fail `heal:` handler — the kernel-level primitive that turns assert-failure into a self-correcting step. Stays in `proposals/` (not `done/`) until the heal handler ships.
**Effort:** M (~1 week — action + executor change)
**Value:** High — turns mooncake from "apply state once" into
"maintain state", which is the kernel-level primitive underneath
pilot agents, fleet drift correction, and self-healing systems.

---

## Problem

Mooncake today is **forward-apply**: take a plan, drive the system
to the desired state, exit. The maintain loop — *check the system
is still in the desired state, and put it back if it isn't* — is a
pattern users have to hand-build with cron + retry + alerting.

A few action handlers (`observe.*`, `wait.*`) come close but stop
short of remediation. The `assert:` field on steps (in some
handlers) checks a postcondition but only fails the plan; it doesn't
recover.

For the directions this kernel naturally wants to grow into —

- **personal provisioning** that keeps reapplying as the environment
  drifts (a package gets uninstalled, a service stops);
- **pilot agents** that maintain invariants while a local LLM
  decides next steps;
- **LAN fleet** where each node is responsible for staying healthy
  without a control plane;
- **self-healing systems** generally —

the missing primitive is the same: **declare an invariant + declare
how to fix it**. Today these are two unrelated steps glued together
by external orchestration.

## Proposal

### `assert` action

A step type whose job is to evaluate a predicate against current
system state and produce a typed pass/fail.

```yaml
- assert:
    name: api_healthy
    check:
      http_ok: https://localhost:8080/healthz
    on_fail: heal_api
```

```yaml
- assert:
    name: nginx_running
    check:
      service_active: nginx
```

`check:` accepts a small grammar of typed predicates — `http_ok`,
`service_active`, `process_running`, `file_exists`, `file_sha256`,
`port_open`, `fact_equals`. Each maps to existing fact / observe
sources; `assert` is the typed pass/fail wrapper around them.

### `heal:` handler on the step

Either inline or by reference:

```yaml
- assert:
    name: nginx_running
    check: { service_active: nginx }
    heal:
      - service: { name: nginx, state: started }
```

Or by named reference to a labeled step elsewhere in the plan:

```yaml
- assert:
    name: api_healthy
    check: { http_ok: https://localhost:8080/healthz }
    on_fail: restart_api    # the label of a downstream step

- service:
    name: my-api
    state: restarted
  labels: [restart_api]
```

### Execution semantics

- Plan-time: planner emits the predicate as a no-op preview ("would
  assert API healthy"). `heal:` actions are planned too so their
  diffs / perms / risk are accounted for, even if not run.
- Apply-time: the predicate is evaluated. If it passes, `heal:` is
  skipped, recap counts `ok`. If it fails, `heal:` runs as a normal
  child plan, the assert is re-checked, and the recap counts `healed`
  (a new counter — small surface, big semantic clarity) or `failed`
  if the heal didn't restore the predicate.
- Replay: history records the predicate result + heal trace, so
  `mooncake history show <id>` tells the full "what drifted, what
  we did, did it work" story.

### Use modes

- **One-shot apply** — `mooncake apply` runs the plan once. Assert +
  heal looks like "fix and verify".
- **Maintain loop** — `mooncake maintain --interval 30s` re-runs the
  asserts on an interval, only running heals when needed. Same plan,
  different driver.
- **Daemon-driven** — `agentd` (fleet stream) consumes a plan and
  runs the maintain loop continuously per node.

The plan is the same artifact across all three. The kernel doesn't
care which driver is calling it.

## Why this is a kernel primitive, not a CLI feature

Self-healing as a userland pattern means each user implements:
- their own scheduler,
- their own predicate types,
- their own "did the heal work?" verification,
- their own audit log,
- their own coordination with mooncake's diff/perms/risk.

Putting `assert` + `heal:` in the kernel collapses all of that into
one declarative step. The maintain loop is just `apply` in a `for`.

## What this doesn't address

- **Backoff / rate-limiting of repeated heals** — flapping services
  shouldn't re-heal forever. Needs a `max_heals_per_window` field;
  pair with a future quota proposal.
- **Multi-step heal sequencing** — can `heal:` itself contain an
  `on_fail`? Probably yes (recursive plans), but worth ratifying
  before shipping. See proposal-15 (`plan` recurse).
- **Inter-node assertion** — "assert all 5 LAN peers think this
  service is up". That's fleet stream, builds on this.

## Field-budget impact

One new universal field on `Step`: `heal:` (sibling to existing
`assert:` field). One new step type: `assert:`. Current count 36 →
37; well under the 40 cap.

The alternative — keep `heal:` inside an `assert:`-only block — is
tempting but limits reuse: `heal:` is useful on any check-like
step (wait, observe). Putting it on `Step` keeps the door open.

## Pairs with

- **Core proposal-04** (typed plan diff) — `assert` contributes its
  predicate to the diff.
- **Core proposal-06** (failed vs error) — `assert` is the canonical
  "query returning false = failure" case; this proposal sharpens the
  taxonomy.
- **Core proposal-14** (`watch`) — event-driven `assert` evaluation
  instead of polled.
- **Fleet stream** — `mooncake maintain` as the daemon mode of the
  same plan; per-node self-healing without a control plane.
