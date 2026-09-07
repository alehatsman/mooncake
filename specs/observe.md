---
id: observe
status: draft
owners: [aleh]
covers:
  - "internal/actions/observe.go"
  - "internal/actions/observe_*/*.go"
---

# Observe (Reading Machine State)

## Intent
`observe.*` is the single typed family for reading machine state from a
playbook. One handler per probe, one shared `ObserveResult` envelope, one
read-only ABI specialization. A step observes; the next step branches on what
it saw via `as:` capture.

Four vocabularies used to answer "is port 5432 open": the `facts`/`snapshot`/
`metrics` CLI reads, the `observe.*` actions, the `wait.*` actions, and
`fleet observe`. The `wait.*` family was the clearest duplication — four
actions that were a poll loop wrapped around a probe `observe.*` already
implemented, with their own timeout parsing, their own backoff, and their own
subtly different defaults.

So polling is not a separate family; it is a modifier. `observe.X` grows
`wait:`, and `wait.*` is gone.

```yaml
- observe.port:
    port: 5432
    wait: { for: 30s, until: open }
```

## Behavior

### The envelope
- WHEN any `observe.*` handler runs, it SHALL return an `ObserveResult`
  carrying `Found`, a typed `Value`, `AsOf`, and a user-facing `Error`, and
  SHALL publish it so `{{ name.found }}` and `{{ name.value.<field> }}` both
  resolve after an `as:` capture.
- WHERE a handler defines what "found" means, it SHALL be the single condition
  the `wait:` modifier polls on — an observe handler has exactly one notion of
  presence, not one per consumer.
- WHEN a handler runs in plan mode, it SHALL return `PlanDeferred` with a
  zero-value typed payload so downstream templates still type-check, and SHALL
  carry the "would observe X" explanation in `Result.Reason` rather than
  reporting a probe failure.

### Read-only ABI
- WHEN an `observe.*` handler declares its ABI, it SHALL report `Changed=false`,
  a `Diff` with `Operation=noop`, `Cost{Risk:1, Resources:0, Bytes:0,
  Reversible:false}`, and `Permissions` with no filesystem writes and no sudo.
- WHERE reversal is concerned, an `observe.*` handler SHALL NOT implement
  `actions.Reverser`. A no-op `Reverse` still satisfies the interface, and the
  registry derives `ImplementsReverse` from that assertion — so implementing it
  makes `mooncake actions list` report `REVERSE yes` for a pure read. A read
  has nothing to undo; that is not the same as being undoable.

### The `wait:` modifier
- WHERE an `observe.*` action's `Found` is meaningful — it can be both true and
  false for the same target — it SHALL accept an optional `wait:` block. Probes
  whose `Found` is constant (`observe.cpu`, `observe.memory`, `observe.disk`,
  `observe.gpu` always observe a host that exists) SHALL NOT accept one, because
  a wait on a constant is either instant or infinite.
- WHEN `wait:` is present, the handler SHALL poll its probe until the condition
  holds or the budget elapses, and SHALL return the LAST observation either way
  so `as:` capture sees real data on both paths.
- WHEN the budget elapses without the condition holding, the step SHALL FAIL,
  naming the condition, the budget, and how many attempts were made. This is the
  behavior `wait.*` had, and it is what makes `wait:` an orchestration
  primitive rather than a slow read.
- WHERE `until:` selects the condition, `found` SHALL mean `Found==true` and
  `gone` SHALL mean `Found==false`. Per-probe synonyms SHALL be accepted so the
  step reads naturally — `open`/`up`/`ready`/`present` for found,
  `closed`/`down`/`stopped`/`absent` for gone — and SHALL be exactly synonyms,
  never a second condition.
- WHERE `for:` and `interval:` are unset, they SHALL default to `60s` and `1s`.
  An `interval:` below `100ms` SHALL be clamped, so a typo cannot turn a wait
  into a busy loop against a remote endpoint.
- WHILE waiting, the handler SHALL honor run cancellation (`ctx.Ctx()`) between
  attempts, so SIGINT during a 5-minute wait aborts promptly.
- WHEN a step with `wait:` runs in plan mode, it SHALL defer like any other
  observation and SHALL NOT poll.

### Retirement of `wait.*`
- `wait.port`, `wait.http`, `wait.file`, and `wait.command` SHALL be removed.
  `observe.port` and `observe.http` already covered the first two;
  `observe.file` and `observe.command` are added here so the last two have
  equivalents rather than being dropped.
- WHEN `observe.file` runs, `Found` SHALL mean the path exists, and — where
  `contains:` is set — that the file also matches it.
- WHEN `observe.command` runs, `Found` SHALL mean the command exited with the
  expected code (default `0`). The command's exit status is data, so a non-zero
  exit SHALL NOT fail the step; only a `wait:` that times out does.

## Non-goals
- `mooncake facts` / `state` / `metrics` CLI reads, and `fleet observe` fan-out.
  Both should route through these handlers rather than reimplementing them, but
  that is the remainder of #180 and not this spec's contract.
- Threshold waits (`until: cpu < 50%`). `until:` polls the handler's single
  `Found` condition; a numeric predicate belongs in `assert` with `wait:`, not
  in a second condition language inside `observe`.
- Reimplementing retry. `retry:` re-runs a failed step; `wait:` polls inside one
  step for a condition that is not yet an error. They compose and neither
  replaces the other.

## Checklist
- [x] Shared `ObserveResult` envelope + `PlanDeferred` + `ObserveValueToMap`.
- [x] Read-only ABI on all nine handlers: noop Diff, Risk=1, no sudo/writes.
- [x] No `observe.*` handler satisfies `actions.Reverser`; `Cost.Reversible`
      is false, guarded across the whole registry.
- [x] `wait:` modifier: `for` / `until` / `interval`, synonyms, interval clamp,
      cancellation between attempts, fail-on-timeout, last observation returned.
- [x] `observe.file` and `observe.command`.
- [x] `wait.port` / `wait.http` / `wait.file` / `wait.command` removed.
- [ ] `wait:` extended to `observe.process` / `observe.service` / `observe.logs`
      (mechanism is shared; only the per-handler wiring is left).
- [ ] `fleet observe` and the `state` CLI route through these handlers instead
      of reimplementing the probes (the remainder of #180).
