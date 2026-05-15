# Spec 58: Fleet Drift — periodic plan-conformance check + `fleet drift` report

**Epic:** Personal Fleet — see [`epics/done/epic-personal-fleet.md`](../../epics/done/epic-personal-fleet.md).
**Sibling framing:** Cluster Management — [`epics/epic-cluster-management.md`](../../epics/epic-cluster-management.md).
Brainstormed in:

- [`clustermanagement/qol-features.md` Tier 2 §9 "Drift detection / reconciliation"](../../clustermanagement/qol-features.md)
- [`clustermanagement/agentic-interface-brainstorm.md` §3 "Self-healing reconciliation loops"](../../clustermanagement/agentic-interface-brainstorm.md)
- [`clustermanagement/issue-11-analysis.md` §10](../../clustermanagement/issue-11-analysis.md) — picked as the
  single highest-leverage item from GitHub issue #11.

**Status:** Draft
**Effort:** M (~8–12 days, three PRs — see Implementation order)
**Value:** High — the single feature that turns Mooncake from
"config management tool" into "fleet operating system." Every other
ambitious cluster-management item (auto-remediation, drift heatmap,
canary verification, AI-proposed mutations) inherits from a working
drift loop. Issue #11 calls drift detection "one of the highest-value
operational features"; this spec is the implementation.
**Depends on:**

- spec-16 unify-dryrun-execute (✅ shipped) — provides the
  non-mutating `InspectPlan` path the drift loop reuses.
- spec-22 phases 3–5 (✅ shipped) — `Permissions` / `Diff` / `Reverse`
  on the priority handler set; `Diff` is what makes drift output
  *typed* rather than free-form prose.
- spec-30 transactions (✅ shipped) — the LIFO rollback infrastructure
  the optional `on_drift: revert` policy reuses.
- spec-43 (✅ shipped) — peer transport + `peers.toml`.
- spec-46 (✅ shipped) — fleet subcommand pattern, exit-code shape,
  `--peer-filter`.
- spec-48 / spec-50 (✅ shipped) — per-host overlays + extended
  filter keys for the spec's per-peer plan-resolution path.

**Does NOT depend on** (intentional carve-out, see Non-goals):

- A periodic snapshot of facts/packages/services (issue #11 item 3).
  v1 drift is *plan-conformance* drift, not *facts-timeline* drift.
  The two ideas are independently useful; this spec is the cheaper,
  more concrete half.

---

## Problem

Today the operator's mental model of "is my fleet still in the shape I
declared?" goes:

```
$ mooncake fleet apply ~/dotfiles/site.yml
[main_pc]    7 changed, 0 failed  ✓
[laptop]     2 changed, 0 failed  ✓
[gpu-box-1]  9 changed, 0 failed  ✓
```

…and then they look away. Three weeks later, somebody manually edits
`/etc/wsl.conf` on `main_pc`. A `pacman -Syu` runs on `laptop` and
swaps the system Python. Disk fills and `systemd` drops the `ollama`
service on `gpu-box-1`. The first signal is a failing `fleet apply`
six weeks later — by which point three different deltas are
intermingled and the operator can't tell which change broke what.

The kernel already has the right primitive: spec-16 `InspectPlan` runs
a plan in non-mutating `ModePlan` and returns per-step verdicts
(`WouldChange=true/false`, `Reason`, `Diff`). What's missing is:

1. **A periodic loop** on agentd that runs `InspectPlan` against the
   last-applied plan and persists the verdict.
2. **A controller-side renderer** (`mooncake fleet drift`) that
   aggregates verdicts across peers.
3. **An on-drift policy** (notify / revert / reapply / nothing) that
   the operator can choose per-machine.
4. **A "what was last applied?"** record per peer. agentd has run
   records but doesn't index them by "this is the current declared
   state for this machine."

The pain is concrete: drift is the most common reason `fleet apply`
takes longer than expected, and there's no UI surface that surfaces
*incremental* drift before it accumulates.

```
# What we want
$ mooncake fleet drift
PEER       PLAN                LAST_CHECKED  CHANGED  REASON
main_pc    machines/main_pc/   3m ago        2/47     /etc/wsl.conf modified externally
                                                       systemd unit ollama disabled
laptop     machines/laptop/    3m ago        0/31     ok
gpu-box-1  machines/gpu-box-1/ 3m ago        1/22     pkg python3 version 3.12.4 → 3.13.1
                                                       (system upgrade)
fleet drift: 2/3 peers drifted, 3 step deltas total
```

---

## Goals

- **G1** Each agentd runs `InspectPlan` against its last-applied plan
  on a configurable interval (default: 1h), storing the latest
  verdict in a peer-local file.
- **G2** `GET /v1/drift` returns the latest verdict for that peer
  (timestamp + per-step inspection slice). 404 if no plan has been
  applied yet.
- **G3** `mooncake fleet drift` fans out across selected peers and
  renders a one-line-per-peer table (PEER, PLAN, LAST_CHECKED,
  CHANGED, REASON). `--peer <name>` prints the full per-step report
  for one peer.
- **G4** `mooncake fleet drift --json` emits one JSON object per peer
  (JSONL) carrying the peer name + the full verdict — same shape as
  `internal/executor/inspect.go:StepInspection` slice.
- **G5** On-drift policy per machine is declared in
  `machines/<name>/fleet.yml`:

  ```yaml
  drift:
    interval: 1h          # 0 = disable
    policy: notify        # notify | reapply | revert | none
    cost_budget: 5        # only reapply if Cost().Sum <= 5 (uses spec-22 Cost())
  ```

  `policy: notify` (default) is the only one with no autonomous side
  effects — emits a structured event on the agentd SSE bus,
  controller can subscribe.
- **G6** "Last-applied plan" is recorded automatically by every
  successful `fleet apply` / `/v1/runs` completion. Drift checks
  reuse the **exact** plan path + vars-files + tags the last run
  used — never a stale earlier shape.
- **G7** A peer running an older agentd that doesn't expose
  `/v1/drift` degrades gracefully — its row shows "drift check
  unavailable (agentd predates spec-58)" and is excluded from the
  drift count. The command does not fail.
- **G8** Exit codes: 0 if every accessible peer is drift-free, 1 if
  any drift is detected or any peer is unreachable, 2 if any drift
  check itself errored. Matches `fleet doctor` / `fleet status`.

**Non-goals:**

- **Facts-timeline drift.** "GPU driver version changed since last
  week" is a different question — it needs a periodic facts/packages
  snapshot (issue #11 item 3). That's its own spec; this one only
  uses the *declared plan* as the reference frame.
- **Automatic remediation by default.** `policy: reapply` and `revert`
  are opt-in per machine. Drift detection that surprises operators
  with autonomous writes is exactly the "infrastructure religion"
  the issue warns against.
- **Cross-peer aggregation for "fleet-wide drift heatmap."** The
  enterprise epic talks about heatmaps + paging alerts. This spec
  ships the data; visualization is later.
- **Replacing `mooncake plan`.** `mooncake plan` is the
  authoring-time check (does this plan make sense?). Drift is the
  ops-time check (does the world still look like the plan said?).
  Same `InspectPlan` underneath, different consumer.
- **TUI / dashboard.** Plain stdout. Same shape as the rest of
  `fleet *`.
- **Continuous-monitoring mode.** v1 is one-shot at the configured
  interval, polled by the operator with `fleet drift`. Live "watch
  the drift loop fire" UX could be a thin wrapper on spec-53
  `fleet watch` later.

---

## Reuse map

**Reused:**

- `internal/executor/inspect.go:InspectPlan` — the substrate. Already
  returns per-step `StepInspection{WouldChange, Checkable, Reason,
  Detail, Diff, Cost}`. The drift loop is a scheduler around it; the
  output shape is *exactly* the existing slice.
- `internal/plan/` — `BuildPlan` from the persisted last-applied
  plan path. Identical to what `fleet apply` does.
- `internal/agentd/store.go:Run` — already records `PlanPath`,
  `VarsFiles`, `Tags`, `Names`, `BaseDir`. Drift loop reads the
  most recent terminal success record and reuses the same fields,
  so a re-inspection sees the same plan shape the apply did.
- `internal/agentd/sse_hub.go` — emits drift events on the existing
  SSE bus. `fleet watch` (spec-53, drafted) shows them next to apply
  events with no special wiring.
- `internal/fleet/machine.go` — already parses
  `machines/<name>/fleet.yml`. Adds the `drift:` block as an
  optional sibling of the existing `phases:` block. Existing
  fleet.yml files load unchanged.
- `cmd/fleet.go` — peer-selection scaffolding (`--peers`,
  `--peer-filter`, `selectPeers`, `peerMatchesFilters`). New
  `fleet drift` subcommand follows the same shape as
  `fleet status` / `fleet doctor`.
- Capability-detection pattern from spec-50/55 — HTTP 404 on the
  new `/v1/drift` endpoint signals "peer is old"; the controller
  emits a warning and excludes the row.
- spec-22 `Cost()` — `policy: reapply` uses `cost_budget` to gate
  automatic re-application. Same `CostEstimate.Sum` the planner
  surfaces in JSON plan output today.

**New:**

- `internal/agentd/drift.go` — the periodic scheduler. Reads
  the latest successful run record, builds the plan, calls
  `InspectPlan`, writes verdict to disk, emits an SSE event.
  Started by `Server.Start` only when at least one machine config
  sets `drift.interval > 0`.
- `internal/agentd/drift_store.go` — `Store.LastApplied(planScope)`
  and `Store.WriteDriftVerdict(verdict)`. Disk shape:

  ```
  <state_dir>/drift/
    last-applied.json        # one record per plan_scope
    verdict-<plan_scope>.json
    verdict-<plan_scope>.json
  ```

  `plan_scope` is a stable hash of (BaseDir, PlanPath, sorted VarsFiles,
  sorted Tags) — so two distinct fleet.yml machines on the same peer
  don't trample each other's verdicts.
- `internal/fleet/transport/client.go:Drift(ctx)` — GET `/v1/drift`,
  returns the verdict struct. New method matches the existing
  `GetFacts` / `GetVersion` pattern.
- `cmd/fleet_drift.go` — the new subcommand. Fan-out, tabwriter
  rendering, JSON mode, single-peer detail mode.

---

## Design

### 5.1 The "last-applied plan" record

After each successful run, the daemon's worker writes
`<state_dir>/drift/last-applied.json`:

```json
{
  "scope": "main_pc/dev",
  "plan_path": "/home/aleh/.../synced/main_pc/dev.yml",
  "base_dir":  "/home/aleh/.../synced/main_pc",
  "vars_files": ["vars.yml"],
  "tags": ["dev"],
  "applied_at": "2026-05-15T11:42:19Z",
  "run_id":    "01J..."
}
```

`scope` is the spec-22 plan scope (machine name + phase name, or
just "default" for non-`machines/` applies). One record per scope;
new applies overwrite. Crash-safe: temp-file + atomic rename
(same pattern as `Store.AppendEvent`).

**Q: what about *failed* apply that left the world in a mixed
state?** A failed run is intentionally NOT recorded as the
last-applied plan — drift inspection against a half-applied plan
is misleading. Operators should `fleet apply` to convergence first.
The drift verdict for a never-converged plan is "no data."

### 5.2 The periodic scheduler

`internal/agentd/drift.go:Loop` is a goroutine started by
`Server.Start` *only if* the daemon has loaded a machine config
with `drift.interval > 0` from `machines/<self>/fleet.yml`.
(`<self>` is the daemon's hostname, same convention `fleet apply
<machine>` already uses.) Otherwise the loop is a no-op and the
endpoint returns 404.

Each tick:

1. Read the most recent record for each scope from
   `last-applied.json`.
2. For each scope, build the plan via the standard
   `planner.BuildPlan` path. Use the recorded base_dir / vars-files
   / tags — never re-resolve "what does the plan look like *now*?",
   because we want to detect external drift, not redeploy.
3. Call `executor.InspectPlan` with `sudoPass=""` (drift never
   prompts; if a step needs sudo it inspects what it can and
   reports `Checkable=false` for the rest).
4. Write the verdict to `verdict-<scope>.json` (atomic rename).
5. Emit `events.EventDriftChecked` on the SSE hub with
   `{scope, changed_count, total, applied_at, checked_at}`.
6. If `policy != none` and `changed_count > 0`, apply the policy
   (next section).

Concurrency: only one tick per scope runs at a time (mutex per
scope). The interval clock is the same `time.Ticker` shape the
SSE hub uses for keepalive — no new dependency.

**Sudo and the inspection's blind spots.** `InspectPlan` against a
non-root daemon will return `Checkable=false` for steps that need
elevated reads (e.g. inspecting a 0600 file owned by root). The
verdict honestly records `Checkable=false`; `fleet drift` renders
them as a separate "UNCHECKABLE" count alongside "CHANGED". This
is a feature, not a bug: drift output that lies about coverage is
worse than drift output that admits its gaps.

### 5.3 On-drift policies

Declared per-machine in `machines/<name>/fleet.yml`:

```yaml
phases:  # existing spec-46 shape, unchanged
  - name: base
    plan: base.yml
  - name: dev
    plan: dev.yml

drift:                  # new in this spec, all fields optional
  interval: 1h          # parsed by time.ParseDuration; 0 disables
  policy: notify        # notify | reapply | revert | none
  cost_budget: 5        # only for policy: reapply
  cooldown: 30m         # don't reapply more than once per cooldown
```

Policy semantics:

- **`none`** — collect verdicts; no side effects. Operator runs
  `fleet drift` manually.
- **`notify`** (default when `interval > 0`) — emits a structured
  SSE event but performs no writes. The controller (or `fleet
  watch`) surfaces it; the human decides.
- **`reapply`** — when drift detected and
  `sum(Cost) <= cost_budget` and the last reapply was more than
  `cooldown` ago, submit a new run with the recorded plan path /
  vars / tags. The run goes through the existing
  `submitRunHandler`, so it shows up in `fleet ps` /
  `fleet history` like any other run. **Drift-triggered runs are
  tagged with `triggered_by=drift` in the run record** so operators
  can filter them.
- **`revert`** — for each drifted step that the handler declared
  `Reverse()` for (spec-22 phase 5), submit a reverse run. Steps
  *without* a `Reverse()` declaration are reported but not
  reverted (consistent with spec-30 transaction semantics: only
  reversible steps participate in LIFO rollback). High-risk
  policy — recommend gating on `cost_budget` and starting with
  `cooldown >= 1h`.

**Why `notify` is the default.** The issue's anti-goal list calls
out "over-centralized architecture" and "infrastructure religion."
Autonomous writes that surprise the operator on drift are exactly
the religion to avoid. `notify` makes the loop visible without
making it autonomous.

### 5.4 Wire shape

New endpoint:

```
GET /v1/drift
GET /v1/drift?scope=<scope>      # one scope, not the whole bundle
```

Response (200):

```json
{
  "scopes": [
    {
      "scope": "main_pc/dev",
      "plan_path": "...",
      "applied_at": "2026-05-15T11:42:19Z",
      "checked_at": "2026-05-15T12:42:21Z",
      "interval_sec": 3600,
      "policy": "notify",
      "total_steps": 47,
      "changed": 2,
      "uncheckable": 3,
      "steps": [
        {
          "step_id": "step-12",
          "action": "file.copy",
          "would_change": true,
          "checkable": true,
          "reason": "destination content drifted",
          "diff": { /* spec-22 typed Diff */ }
        },
        /* ... */
      ]
    }
  ]
}
```

404 when:

- No machine config sets `drift.interval > 0` (daemon never started
  the loop).
- The requested scope has never had an applied plan.

Daemons predating spec-58 return 404 on `/v1/drift` (no route
registered). The controller treats 404 as "feature unavailable" and
renders an explanatory row.

### 5.5 Controller-side rendering

`mooncake fleet drift` defaults to the summary table:

```
PEER       SCOPES         CHANGED  UNCHECK  CHECKED        POLICY
main_pc    1              2        0        3m ago         notify
laptop     1              0        0        3m ago         notify
gpu-box-1  2              1+0      0        3m ago         reapply
unreached  -              -        -        -              -
fleet drift: 2/3 peers drifted (3 deltas), 1 peer unreachable, 1 peer uncheckable
```

`fleet drift --peer <name>` switches to detail mode and shows the
per-step inspection (PASS / CHANGED / UNCHECKABLE with reason +
diff). Diff rendering reuses whatever `mooncake plan --json` /
spec-22 surfaces.

`fleet drift --json` is JSONL (one peer per line) — same shape
`fleet doctor --json` uses, so jq pipelines that work for doctor
work for drift.

`fleet drift --since <duration>` shows only verdicts checked in
the last N minutes (for catching "did the loop fire after I made
this change?" without `watch` shenanigans).

`fleet drift --refresh` forces every selected peer to re-run the
inspection *now*, ignoring its interval cache. Useful in incident
response: "did my last apply actually take?"

### 5.6 Interaction with `fleet apply`

`fleet apply` writes the last-applied record on success. **Two
caveats:**

- `fleet apply --dry-run` does NOT update the record (correct: no
  state was changed, so the prior "applied plan" is still the
  reference frame).
- `fleet apply --step-filter name=foo` (a partial apply) updates
  the record for *only the steps that ran*. This is the harder
  case — partial-apply semantics. v1 punts: the record is updated
  iff `--step-filter` was empty *and* `--tags` matches the
  previously-recorded tags. Otherwise the daemon emits a warning
  ("partial apply; drift reference frame unchanged") and leaves
  the old record alone. A real story for partial-apply provenance
  is its own design problem (see Open questions §11.3).

### 5.7 MCP exposure (optional, see Open questions)

If we expose drift through MCP, the natural tool is
`fleet.drift(peer?, scope?, refresh?)` returning the structured
verdict. The LLM use case ("which peer is drifted? what's the
diff?") falls out of MCP federation (issue #11 / agentic doc §6)
but doesn't block v1.

---

## Implementation order (3 PRs)

**PR A — last-applied record + inspect-from-record (~3 days):**

- Daemon writes `last-applied.json` on every successful run.
- New CLI command `mooncake drift inspect` (single peer / local
  only) loads the record, calls `InspectPlan`, prints the verdict.
  No HTTP, no fleet yet.
- Tests: the round-trip from `fleet apply` → record-on-disk →
  inspect → expected verdict.

This is the minimum-viable kernel of the feature. Lands solo and
remains useful even if PR B is delayed.

**PR B — periodic loop + `/v1/drift` endpoint + `fleet drift` CLI (~5 days):**

- Add `drift:` block to `machines/<name>/fleet.yml` parser.
- Start the loop in `Server.Start` when any scope is configured.
- Persist verdicts; expose `/v1/drift`.
- Add `Client.Drift(ctx)` to transport.
- New `cmd/fleet_drift.go` (summary + `--peer` detail + `--json`
  + `--refresh` + `--since`).
- Tests: parameterized over policy=`notify`/`none`, interval
  honoring, capability-detection (old daemon → "unavailable" row).

After PR B, the operator-facing UX is complete for `policy: notify`
and `policy: none`. These are the safe defaults; everything else
is opt-in.

**PR C — autonomous policies (`reapply`, `revert`) + cost gates (~3 days):**

- Wire `policy: reapply` to submit drift-triggered runs through
  `submitRunHandler` with `triggered_by=drift` in the run record.
- Wire `policy: revert` to assemble a reverse plan from the
  inspection's `Reverse()`-declared steps and submit it the same
  way.
- Enforce `cost_budget` + `cooldown` before any autonomous run.
- Emit `events.EventDriftAction` on the SSE bus so `fleet watch`
  shows what the loop did.
- Tests: cost budget cap, cooldown, `Reverse()`-missing steps are
  reported-but-not-reverted, drift-triggered runs are filterable
  in `mooncake history` / `fleet ps`.

PR C is the riskiest of the three. Hold for review until PR A + B
have soaked on a real fleet for a week. Until C lands, drift
remains a *purely observational* feature — by design.

---

## Open questions

1. **One scope or many per peer?** Today a peer can host multiple
   machine configs (`machines/main_pc/fleet.yml` and
   `machines/main_pc-secondary/fleet.yml` if the operator ever
   wants overlapping shapes). The spec assumes one record per
   scope and the daemon iterates all configured scopes. Edge case:
   two scopes that touch the same files will report drift against
   each other. Probably fine — operators don't actually structure
   things this way — but worth a paragraph in the kernel docs.

2. **Should `--refresh` block on the loop?** If the loop is mid-tick
   when `--refresh` arrives, do we coalesce? Initial design: yes,
   coalesce via the per-scope mutex; `--refresh` returns whichever
   verdict the in-flight tick produces. Latency budget: one
   `InspectPlan` invocation, typically <2s for a 40-step plan.

3. **Partial-apply provenance.** The spec punts on the
   `fleet apply --step-filter` case. A clean fix needs the daemon
   to track per-step provenance ("step `foo` was last applied by
   run X at time T"), not per-plan. That's larger than this spec
   wants to be; punt to a future spec.

4. **What about plans loaded via `!secret`?** Spec-23 `!secret`
   resolves at plan-load time. If the secret changes between
   apply and drift check, the inspection will report a drift on
   any secret-bearing step. Correct behavior, but noisy. Possible
   mitigations: per-step "ignore drift on this field" annotation,
   or hash-only secrets in the recorded plan. Punt to a future
   spec; document the gotcha.

5. **MCP surface.** Worth shipping in PR B for the LLM use case,
   or hold until federated MCP (agentic doc §6) lands? Initial
   leaning: hold. Drift's value with one peer + LLM is small;
   value with a fleet + federated MCP is large. Build the
   foundation first.

6. **Should the daemon emit an `EventDriftCleared` when a previously
   drifted scope returns to clean?** Useful for "did the reapply
   take?" but easy to leak via missed transitions. Initial design:
   yes, emit it — operators wired up to alerts need the all-clear
   signal. Keep the event flat ({scope, cleared_at}); consumers
   correlate with the prior EventDriftChecked.

7. **Daemon-internal counters.** Should `/v1/version` start
   exposing `drift_loop_alive` / `last_drift_check_ago_sec` so
   `fleet doctor` can flag a stalled loop? Probably yes, but
   that's `fleet doctor`'s spec (55), not this one — file as a
   follow-up.

8. **What if the recorded plan path no longer exists on the peer?**
   (Operator removed a file between apply and drift check.) Daemon
   reports a `verdict_error: "plan_path missing"` and the
   controller renders it. Don't fall back to an older record — the
   reference frame must always be the most-recently-applied state.

9. **Drift on plans that include `transaction:` blocks.** Spec-30
   transactions are a planner-level construct; `InspectPlan`
   already walks transactional steps the same way it walks
   non-transactional ones. No special handling needed; verify in
   PR A tests.

10. **Should `fleet drift` exit code 1 propagate through CI?**
    Probably yes — drift in CI is a regression signal. Document
    this in the spec output examples so operators wire it up.

11. **Daemon-internal health surfacing.** spec-55 §"Open questions"
    asks the analogous question about doctor: should the daemon
    surface queue depth + recent run errors? Drift loop status
    belongs in the same bucket. Defer to spec-55 to settle the
    pattern; this spec adopts whatever shape spec-55 ships.

---

## Cross-references

- **Issue #11** — items 3 (facts cache, deliberate non-dependency),
  10 (drift detection, this spec), 11 (explain node/resource, depends
  on this spec's per-step verdict storage as raw material).
- **`docs-working/clustermanagement/issue-11-analysis.md`** —
  picked this item as the highest-leverage candidate.
- **`docs-working/clustermanagement/qol-features.md` §9** —
  original brainstorm. This spec answers most of its open
  questions but stays scoped to one feature; the "reapply on
  drift" policy from §8 (`fleet schedule`) lives here as
  `policy: reapply`.
- **`docs-working/clustermanagement/agentic-interface-brainstorm.md` §3** —
  the "self-healing reconciliation loops" framing. This spec is
  the kernel that framing needs; per-peer micro-operators
  (brainstorm §8) become possible *after* drift detection is the
  signal they react to.
- **`docs-working/specs/personal-fleet/spec-53-fleet-watch.md`** —
  drafted. `fleet watch` should surface the new
  `EventDriftChecked` / `EventDriftAction` events with no special
  wiring; verify in PR B integration tests.
- **`docs-working/specs/done/spec-30-transactions.md`** — the LIFO
  rollback infrastructure `policy: revert` reuses.
- **`docs-working/specs/done/spec-16-unify-dryrun-execute.md`** —
  `InspectPlan` itself.
- **`epics/epic-cluster-management.md`** — the enterprise-flavored
  drift-heatmap UX builds on this spec's data. Keep this spec
  scoped to personal-fleet; heatmap is a future spec.
