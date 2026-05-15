# Cluster Management — Quality-of-Life Features

Concrete near-term features that would improve daily fleet ops, ranked
by leverage given the existing agentd infrastructure (TCP listener +
bearer auth + SSE event hub + run-submit + multiplexer + per-host
overlays + extended filter keys + `fleet discover` with mDNS).

Each item is small (typically ~100–200 LOC), high-frequency-use, and
falls out naturally from the existing agentd surface. Tier-1 picks
share infrastructure (SSE multiplex, fan-out + table rendering), so a
bundle of two or three lands in a day.

---

## Tier 1 — small, daily-use, build on what's already there

### 1. `fleet exec '<command>'` — ad-hoc shell across N peers

The single biggest fleet DX gap. Think `pssh`, `ansible -m shell`, or
the muscle-memory equivalent of `ssh peer 'cmd'` done N peers at once.

- Transport + multiplexer already exist; just need a one-shot
  run-submit path that doesn't need a plan file.
- Could land as a special inline plan synthesized from the command
  (single `shell` step), submitted to each peer's existing
  `/v1/runs` endpoint, output interleaved through the existing
  multiplexer.
- Estimated ~200 LOC.

Why high-leverage: when something's wrong on the fleet, the first thing
you reach for is "what does `systemctl status sshd` show on every
box?" — and writing a plan file for that is friction operators avoid.

### 2. `fleet watch` — live SSE event stream across peers

"Show me whatever the fleet is doing right now," without needing to
have started an apply yourself.

- Subscribes to every peer's SSE stream simultaneously, renders
  events through the same multiplexer `fleet apply` uses.
- Useful when there's drift, a scheduled job firing, or another
  operator/agent driving the fleet concurrently.
- Estimated ~150 LOC.

### 3. `fleet ps` — list in-flight runs across peers

`GET /v1/runs?status=running` fan-out + tabwriter rendering.

- Mirrors `fleet status` table shape (HOST RUN_ID STARTED_AT
  CURRENT_STEP).
- Useful for "is anyone else applying right now?" or "which peer is
  taking so long?"
- Estimated ~100 LOC.

### 4. `fleet doctor` — `mooncake doctor` fleet-wide

Run the kernel's existing doctor (16 health checks) on every peer,
aggregate, render a unified pass/fail table.

- Kernel already has doctor; this is a fan-out wrapper.
- Surfaces "this peer is on a stale agentd version", "this peer's
  systemd is misconfigured" before they bite you.
- Estimated ~120 LOC.

### 5. `fleet metrics` — fleet-wide CPU/memory/disk roll-up

Per-agent `/v1/metrics` already exists (spec-49). Controller-side
aggregate + table renderer.

- "Is any peer's disk full?", "Which peer is hot?"
- Estimated ~120 LOC.
- Pairs naturally with `fleet doctor` — same fan-out shape.

---

## Tier 2 — bigger but strategically meaty

### 6. `fleet rollback` — apply the previous successful plan

Built on spec-30 PR B's executor (LIFO rollback). The agent-safety
story extended to fleet scale: *"whatever just changed across the
fleet — undo it."*

- For each peer, look up the most recent successful run; produce its
  reverse transaction; apply.
- Coordinated across peers? Per-peer is straightforward; cross-peer
  ordering (rollback in reverse phase order) needs design work — pairs
  with `mooncake fleet apply <machine>` semantics.

### 7. `fleet history` — aggregated run history across peers

`mooncake history` already shows local runs. Fleet version asks
"what changed across all my boxes last week?"

- Fan-out `GET /v1/runs` + merge + sort by time.
- Optional controller-side cache for cross-session queries.
- `--peer <name>`, `--since <duration>`, `--grep <pattern>` flags.

### 8. `fleet schedule` — agentd-side cron persistence

*"Apply this plan daily at 3am"* without setting up cron on every box.

- agentd holds the plan + schedule, fires on its own timer, emits
  results through the normal SSE stream.
- Persistence: per-peer state file alongside run records.
- Bigger of the tier; needs daemon state-machine work, lifecycle
  (what happens when the plan is updated), conflict handling.

### 9. Drift detection / reconciliation

agentd periodically runs spec-16 `inspect` mode, pushes drift events
to the controller. *"main_pc-win has drifted on 3 steps since the
last apply."*

- Architectural change: agentd grows a periodic loop.
- Pairs naturally with `fleet schedule` (#8) — a "reapply on drift"
  policy.
- This is where mooncake starts to feel like Kubernetes
  reconciliation rather than ansible.

---

## Recommendation

Start with **`fleet exec`** + **`fleet watch`** as a cooldown bundle.

- They share infrastructure (SSE multiplex).
- Together they make the fleet feel *alive* in a way `fleet apply`
  alone doesn't.
- ~350 LOC total, no new dependencies, lands in a day.
- The tradeoff: neither is strategically load-bearing — they're pure
  QoL — so they sit alongside the agent-safety track rather than
  advancing it.

For follow-on bundles, the Tier-1 fan-out features (`ps`/`doctor`/
`metrics`) cluster nicely into a second "fleet observability" bundle.
Tier-2 items want their own scoping passes.
