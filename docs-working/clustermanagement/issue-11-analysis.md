# Issue #11 — Cluster-management capabilities analysis

**Source:** [#11 Explore cluster-management capabilities enabled by mooncake agentd](https://github.com/alehatsman/mooncake/issues/11)
**Analyzed against:** master @ `c6f6838` (2026-05-15)
**Author:** alehatsman, 2026-05-15

The issue is a 20-item brainstorm of "if agentd is on every box, what
fleet operations become possible?" — visionary, not specced. This doc
walks each candidate against the current code, marks **state** (shipped /
drafted / not started), and flags where the existing docs-working
material already covers the same idea so we don't double-spec.

The headline finding: **roughly half of the issue is already done or
already drafted under `docs-working/`**. The other half splits cleanly
between "small QoL wrappers around existing primitives" and "real new
mechanism" (cordon/drain, drift loop, maintenance windows, policy gates,
power control). The genuinely new strategic bet — typed observability —
is *not* on the issue's list but lives in
[`agentic-interface-brainstorm.md`](agentic-interface-brainstorm.md).

---

## At a glance

| # | Capability | State | Pointer |
|---|---|---|---|
| 1 | Fleet inventory | 🟡 partial | `fleet facts --query`, `fleet status` |
| 2 | Health model | 🟡 partial | `fleet doctor`, `fleet status` |
| 3 | Facts cache | 🟡 partial | live `/v1/facts`; no periodic snapshot |
| 4 | Tags & selectors | ✅ done | spec-48 + spec-50 |
| 5 | Drain / cordon | ❌ not started | new mechanism |
| 6 | Rolling apply / waves | 🟡 partial | `fleet apply <machine>` phases |
| 7 | Canary execution | ❌ not started | adjacent to waves |
| 8 | Remote run logs / events | ✅ done (+ draft) | `fleet logs`, spec-53 watch |
| 9 | Artifact collection / sync | 🟡 partial | local jsonl; no fan-in |
| 10 | Drift detection | 📝 drafted | [spec-58](../specs/personal-fleet/spec-58-fleet-drift.md) |
| 11 | Explain node/resource | ❌ not started | brainstormed in agentic doc §4 |
| 12 | Agentd self-update | ✅ done | `fleet upgrade` |
| 13 | Typed remote execution | ✅ done (+ draft) | apply; spec-52 raw `exec` |
| 14 | Workload placement | ❌ not started | new |
| 15 | Maintenance windows | ❌ not started | new |
| 16 | Reboot orchestration | ❌ not started | new (depends on cordon/drain) |
| 17 | Secret distribution | 🟡 partial | `!secret` resolver shipped, fleet story open |
| 18 | Policy gates | ❌ not started | brainstormed in agentic doc §5 |
| 19 | Fleet topology | ❌ not started | new |
| 20 | Power control (WoL/IPMI) | ❌ not started | new |

Legend: ✅ shipped · 🟡 partial / shipped-but-narrow · ❌ not started · draft = spec exists, not implemented.

---

## Per-capability detail

### 1. Fleet inventory  🟡 partial

**Issue asks:** `mooncake fleet inventory` — every node reports
hostname/os/cpu/ram/disk/gpu/cuda/driver/packages/services/ports/tags/last_seen.

**Today:**

- `internal/facts/` collects hostname, os, distro, kernel, cpu/ram,
  toolchains, services. Exposed at `/v1/facts` and via
  `mooncake fleet facts <peer>` / `--query <key>`.
- `internal/metrics/` collects cpu/mem/disk/net/gpu (Linux) +
  per-OS temp probes. Exposed at `/v1/metrics`.
- `fleet status` reads version + queue + running — no merged
  inventory view.
- Peer-side `last_seen` was just landed (`16e54d2`).

**Gap:** a single command that merges `version + facts + metrics +
last_seen` into one fleet-wide table. Per-peer pieces all exist —
this is a controller-side renderer, ~100–150 LOC. Drafted at
high-level in [`qol-features.md` Tier 1 §5 (`fleet metrics`)](./qol-features.md#5-fleet-metrics)
and §2/§3 (`fleet watch`/`fleet ps`), but no spec named
"inventory" yet.

### 2. Health model  🟡 partial

**Issue asks:** reachable / agentd_alive / disk_ok / gpu_ok /
load_ok / services_ok / last_run_status — operational readiness,
not Prometheus.

**Today:**

- `fleet doctor <peer>` (`cmd/fleet_doctor.go`, just landed with
  SSH fallback in `35c9897`) walks a per-peer probe ladder:
  Resolve → TCP → HTTP → Auth → Facts.
- `internal/doctor/` has ~16 *local* checks (install / system /
  state / presets / tools / project / services).
- `fleet status` shows reachable + queue depth across peers.

**Gap:** the kernel doctor isn't yet fanned out across peers —
that's exactly **spec-55 (`fleet doctor` fleet-wide)** in
[`personal-fleet/spec-55-fleet-doctor.md`](../specs/personal-fleet/spec-55-fleet-doctor.md).
Drafted, not implemented. When that lands, items 2 ("health model")
and 1 ("inventory") collapse into one well-defined surface.

### 3. Facts cache  🟡 partial

**Issue asks:** agentd periodically snapshots facts.json /
packages.json / services.json / gpu.json / ports.json for faster
planning, offline reasoning, drift detection, explainability.

**Today:**

- Facts are computed on-demand at `/v1/facts`; no scheduled
  snapshot.
- `internal/facts/cache.go` exists — but it's an *in-process*
  TTL cache, not a persisted timeline.

**Gap:** "facts cache as a time-series" is the substrate the
drift loop (#10) and the explain command (#11) both need. Worth
designing once with both consumers in mind. Same shape as the
"federated inventory graph" item in
[`agentic-interface-brainstorm.md` §2](./agentic-interface-brainstorm.md).

### 4. Tags & selectors  ✅ done

**Issue asks:** `tags: { role, gpu, os, location }` plus
`--selector 'role=inference,gpu=rtx5090'`.

**Today:**

- spec-48 — per-host overlays + `tags` field on `Peer` (shipped).
- spec-50 — extended filter keys: `tag=`, `name=`, `os=`, `role=`
  with AND-within / OR-across semantics
  (`cmd/fleet.go:peerMatchesFilters`). `Peer.Roles` is a distinct
  field for "what's this machine for" vs the free-form `Tags`.
- All `fleet *` commands accept `--peer-filter` and `--peers`.

**Verdict:** done at the level the issue asks for. Future
extensions (GPU model as a structured selector vs a tag string)
are inventory-shaped, not selector-shaped.

### 5. Drain / cordon  ❌ not started

**Issue asks:** `node cordon` (prevent new mutations) +
`node drain` (gracefully stop / migrate workloads).

**Today:** nothing. agentd has no "accepting new runs?" flag.
`worker.go` accepts every queued run; no admission control.

**Effort:** medium. Requires:

- agentd state flag (`cordoned bool`) persisted across restarts.
- `POST /v1/admission/cordon` + `uncordon` endpoints.
- `submitRunHandler` returns 503 when cordoned (with override
  header for emergency).
- Controller-side `mooncake fleet cordon|uncordon <peer-filter>`.

Drain is harder — it requires a notion of "managed workloads"
that can be stopped, which mooncake doesn't have today
(applications are configured-and-forgotten, not lifecycle-managed
by mooncake). For v1, cordon-only is probably enough; drain can
wait for a real workload model.

### 6. Rolling apply / waves  🟡 partial

**Issue asks:** `fleet apply upgrade.yml --wave-size 2 --health-gate`.

**Today:**

- `fleet apply <machine>` exists (PR landed `35f21a9`/`beb495e`)
  and drives **ordered phases per machine** via
  `machines/<name>/fleet.yml`. That's a 1-D wave with size=1.
- `fleet apply` across peers (no machine) is parallel — no wave
  semantics.

**Gap:** generalize machine phases to *fleet-level* waves.
Mechanically: split the peer list into wave-sized chunks, apply
each chunk in parallel, run a health gate (which doctor command?
which exit code?) between chunks, abort if the gate fails. ~200
LOC on top of existing `apply.go`. Pairs naturally with **#2
(health model)** as the gate.

### 7. Canary execution  ❌ not started

**Issue asks:** `fleet apply deploy.yml --canary gpu-box-1`.

**Today:** nothing. A canary is a wave of size 1 with an explicit
target and a gate; this falls out of #6 with a different CLI
shape.

**Verdict:** ship #6 first, then expose `--canary` as a thin
flag on top.

### 8. Remote run logs / event streaming  ✅ done (+ draft)

**Issue asks:** `fleet logs --run run-123 --follow` — agentd
streams stdout/stderr/events/step status/rollback status.

**Today:**

- `fleet logs <peer>` reattaches to a peer's latest run.
- `fleet logs --all` reattaches across all peers (note: closes
  when all selected runs reach terminal — that's the limitation
  spec-53 addresses).
- SSE hub in `internal/agentd/sse_hub.go` emits per-step events
  with structured fields.
- `--run <id>` is already supported on `fleet logs`.

**Drafted:** spec-53 `fleet watch` — always-on multi-peer stream
that survives terminal states and picks up new runs.

### 9. Artifact collection / synchronization  🟡 partial

**Issue asks:** plan.json / events.jsonl / results.json / diffs/
stdout/ stderr/ facts-before.json / facts-after.json, stored
locally and optionally synced.

**Today:**

- agentd persists per-run state via `internal/agentd/store.go`
  and `jsonl_sink.go` (run records + JSONL event log).
- `internal/runlog/` is the local-mode equivalent.
- No fan-in: the controller doesn't pull artifacts back after a
  run.

**Gap:** controller-side pull (`GET /v1/runs/{id}/artifacts`,
streamed) + local archival path. The `facts-before` /
`facts-after` shape needs the facts-cache substrate from #3.

### 10. Drift detection  📝 drafted as spec-58

**Issue asks:** `fleet drift` — compare last applied plan vs
current facts.

**Today:**

- spec-16 `inspect` mode runs *on demand*: re-evaluate without
  mutating.
- No periodic loop in agentd. No "what was last applied?" record
  beyond run history.

**Status:** this is the highest-strategic-value item on the
list — it's what "fleet operating system" really means. Already
brainstormed in two places:

- [`qol-features.md` Tier 2 §9](./qol-features.md#9-drift-detection--reconciliation)
- [`agentic-interface-brainstorm.md` §3 "Self-healing reconciliation loops"](./agentic-interface-brainstorm.md)

Both notes argue this should depend on spec-30 (transactions)
which has shipped. **Drafted in [`spec-58`](../specs/personal-fleet/spec-58-fleet-drift.md)**:
periodic `InspectPlan` loop on agentd, `/v1/drift` endpoint,
`mooncake fleet drift` renderer, per-machine `drift:` block with
`notify | reapply | revert | none` policies, three-PR rollout
keeping the autonomous policies behind PR C.

### 11. Explain node/resource state  ❌ not started

**Issue asks:** `fleet explain gpu-box-1 docker` — show install
provenance, drift, what depends on it.

**Today:** nothing. The data isn't even there — runs aren't
indexed by "what they touched."

**Gap:** needs the artifact-pull pipeline from #9 + a content
index keyed by `(host, resource_name)`. The `Reverse()` method
from spec-22 phase 5 gives us the typed before/after diff per
step — that's the raw material. Still: real new mechanism. The
"agentic" framing in
[`agentic-interface-brainstorm.md` §4 "Conversation as the unit
of work"](./agentic-interface-brainstorm.md) names this
`fleet why`.

### 12. Agentd self-update  ✅ done

**Issue asks:** `fleet upgrade-agentd` with canary/wave semantics.

**Today:** `mooncake fleet upgrade` (`cmd/fleet_upgrade.go`,
shipped `534044b`/`96d3bfb` with Windows hardening in `fd115ab`).
Linux uses `syscall.Exec` for in-place replacement; Windows uses
MoveFile-the-running-exe + detached helper to drive scheduled-task
Stop/Start. Controller polls `/v1/version` until daemon returns.

**Gap relative to the issue:** wave/canary semantics aren't
expressed *in `fleet upgrade`* — but the controller can call it
peer-by-peer manually. Once #6 (waves) lands as a general
mechanism, `fleet upgrade` should pick it up.

### 13. Typed remote execution  ✅ done (+ draft)

**Issue asks:** prefer `fleet exec --action observe.gpu` /
`os.service` over raw SSH.

**Today:**

- `fleet apply <plan>` is the typed path — every step is an
  action with a typed handler.
- The MCP server fronts the same surface (`internal/mcp/`).
- Raw shell *is* available via the `shell` action inside a plan.

**Drafted:** spec-52 `fleet exec '<command>'` — exactly the
"raw shell as one-shot" wrapper the issue wants to make explicit
and high-risk. Note: the "typed" prefer-path is already canonical;
spec-52 makes the raw escape hatch ergonomic, not the default.

### 14. Workload placement  ❌ not started

**Issue asks:** `fleet place --requires gpu.memory>=24GB --service ollama`
— placement recommendation, not a full scheduler.

**Today:** facts include GPU but there's no placement engine.

**Gap:** real new feature. Depends on #3 (facts cache as queryable
substrate) to be useful. Probably gated until typed observability
(`agentic` doc §1) lands — then "GPU memory >= 24GB" is a typed
query against a typed facts schema, not a string match.

### 15. Maintenance windows  ❌ not started

**Issue asks:** declare allowed windows; risky mutations outside
require explicit override.

**Today:** nothing. agentd accepts runs at any time. No notion of
"risky."

**Gap:** new mechanism but well-scoped. Could land as:

- agentd config: `maintenance.allowed = ["Sat 01:00-04:00"]`.
- new `Permissions().Risky` bit on handlers (spec-22 phase 3
  already declared `Permissions()` across the priority set —
  this would be one more bit).
- submitRunHandler refuses risky plans outside the window
  unless `--override-maintenance` is set.

Brainstormed in [`agentic-interface-brainstorm.md` §7
"Coordination primitives"](./agentic-interface-brainstorm.md)
("time-windowed maintenance").

### 16. Reboot orchestration  ❌ not started

**Issue asks:** cordon → pre-reboot hook → reboot → wait for
reconnect → health check → uncordon → continue.

**Today:** nothing. No reboot action; agentd has no reconnect-
detection loop.

**Gap:** depends on #5 (cordon), #2 (health gate), #6 (wave
machinery), and #12 (already-done reconnect polling pattern).
The blocker is cordon; everything else falls out.

### 17. Secret distribution  🟡 partial

**Issue asks:** agentd resolves secrets locally (env / file / age
/ vault / 1password); controller avoids plaintext.

**Today:**

- spec-23 §3 `!secret` YAML tag shipped — resolves at *load*
  time on whichever box is loading the plan. The controller
  loads the plan to ship it across.
- No agentd-side secret resolver.

**Gap:** for fleet ops, plans should be shipped with placeholder
secrets and resolved per-peer on the peer side. Architecturally
this is a bigger change — `!secret` becomes a deferred-resolution
token in the wire plan, agentd resolves it from its local
provider chain before the executor sees it. Real spec needed.
The capability-scoped trust thread in
[`agentic-interface-brainstorm.md` §5](./agentic-interface-brainstorm.md)
is adjacent.

### 18. Policy gates  ❌ not started

**Issue asks:** node-local policy DSL: deny sudo outside
maintenance, deny `/etc/ssh/*` writes, deny network ops on
airgapped nodes.

**Today:** nothing. Agentd applies whatever the controller
submits, given the bearer token.

**Gap:** policy engine on the agentd side. Brainstormed in
[`agentic-interface-brainstorm.md` §5 "Capability-scoped
trust"](./agentic-interface-brainstorm.md) and listed in
`streams.md` under unwritten Stream-2 specs ("policy DSL", "per-
action quotas", "egress policy", "sandbox mode"). Strategic — it's
part of the safe-agent-runtime wedge. Probably wants to be
several specs.

### 19. Fleet topology  ❌ not started

**Issue asks:** `fleet topology` — controller, child nodes,
depends_on / same_lan / storage_shared / wol_capable metadata.

**Today:** nothing. Peers are a flat list in `peers.toml`.

**Gap:** new metadata layer + a tree renderer. Most of the value
is in the metadata schema, not the renderer. Probably wait until
there's a concrete consumer (placement #14, power-graph #20)
before designing the schema.

### 20. Power control (WoL / IPMI)  ❌ not started

**Issue asks:** WoL, SSH shutdown, agent graceful stop; later
IPMI / Redfish / BMC.

**Today:** nothing. agentd has no power hooks; no out-of-band
control plane.

**Gap:** WoL is small (a UDP magic packet — ~50 LOC + a
`wol_mac` field per peer). SSH shutdown reuses spec-44's SSH
transport. The bigger story (IPMI/Redfish) is explicitly
*below the OS* — which the `epic-cluster-management.md`
clean-boundary section ("Mooncake manages everything above
the OS") deliberately punts on. Recommendation: ship WoL +
SSH shutdown for the personal-fleet use case; leave BMC to
out-of-tree integrations.

---

## What's missing from the issue

Two themes that don't appear in #11 but matter strategically:

1. **Typed observability primitives** (`observe.*` / `probe.*` /
   `measure.*`) — the mirror of the typed-mutation ABI.
   Already argued as the highest-leverage next bet in
   [`agentic-interface-brainstorm.md` §1](./agentic-interface-brainstorm.md).
   It's the substrate for the issue's drift detection (#10),
   placement (#14), and explain (#11). Without it, those three
   live on free-form text parsed out of `shell` actions.

2. **Federated MCP** — controller-side MCP that aggregates each
   peer's existing MCP server, so an LLM connects to *one*
   endpoint and operates the fleet.
   [`agentic-interface-brainstorm.md` §6](./agentic-interface-brainstorm.md).
   Demo-friendly, lower-foundational than (1).

The issue's framing is operator-centric ("a human running fleet
commands"); both gaps are agent-centric ("an LLM reading + writing
fleet state through typed surfaces"). They're complementary, not
substitutes — but worth keeping side-by-side when prioritizing.

---

## Recommended next reads

- [`epics/epic-personal-fleet.md`](../epics/epic-personal-fleet.md)
  — the solo-dev framing this issue largely inhabits.
- [`epics/epic-cluster-management.md`](../epics/epic-cluster-management.md)
  — the platform-team framing for the bigger fleet items
  (hub, RBAC, drift heatmaps, AI remediation).
- [`clustermanagement/qol-features.md`](./qol-features.md) —
  the near-term Tier 1/2 list that overlaps with issue items
  1, 2, 5, 6, 8, 10.
- [`clustermanagement/agentic-interface-brainstorm.md`](./agentic-interface-brainstorm.md)
  — strategic bets (typed observability, federated MCP,
  reconciliation loops, capability-scoped trust) the issue
  doesn't name directly.
- [`PROGRESS.md`](../PROGRESS.md) rev12 — current shipped state
  (13/14 personal-fleet PRs).

## Suggested specing order

Given what's already in flight, the cleanest spec sequence to
absorb issue #11 is:

1. **spec-55 fleet doctor (drafted)** — fan out the kernel doctor,
   land item 2 cleanly. Unblocks item 6's health gate.
2. **spec-52 fleet exec (drafted)** — close the operator-DX gap;
   touches item 13 (the typed/raw boundary).
3. **spec-53 fleet watch (drafted)** — item 8 polish.
4. **New: `fleet inventory`** — one renderer over existing
   `/v1/facts` + `/v1/metrics` + `/v1/version`. Item 1.
5. **New: cordon/admission control** — item 5. Unblocks 6/7/16.
6. **New: rolling apply / waves** — item 6, picks up cordon.
7. **New: facts cache + drift detection** — items 3 + 10,
   designed together. The biggest single feature on the list;
   wants its own design pass.

Items 14 (placement), 17 (fleet secrets), 18 (policy), 11
(explain) want typed observability first; treat them as
post-observability specs rather than pre-requisite ones.

Items 19 (topology), 20 (power) and 15 (maintenance windows) are
opportunistic — small, low-priority, lands when somebody actually
needs them.

Items 9 (artifacts) and 12 (agentd self-update) are largely
covered; revisit only when a concrete consumer surfaces.
