# Spec 42: `fleet status`, `fleet logs`, `fleet facts`

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md), sub-epic P4.
**Status:** Draft
**Effort:** S (~3 days)
**Value:** Medium — the "at-a-glance" command that lets you check on the
fleet between applies. Reuses spec-43 transport almost entirely; this is
mostly CLI surface.
**Depends on:** spec-43 (transport + peers.toml).

---

## Problem

After spec-43 ships, a controller can apply plans across peers. It cannot:

1. See **what's healthy** without running an apply.
2. **Reattach** to a run that's already in flight (e.g. left running when
   you `^C`'d the multiplexer).
3. **Read live facts** from a peer without applying anything.

These three are the inspection surface that makes the fleet feel like one
machine rather than four. Status answers "is everything good"; logs answers
"what is X doing right now"; facts answers "what does X look like".

---

## Goals

- **G1** `mooncake fleet status` prints a one-line-per-peer table covering
  reachability, OS, mooncake version, queue depth, and most recent run
  outcome.
- **G2** `mooncake fleet logs <host>` attaches to that peer's most recent
  in-flight run (or the last finished run if none active), streaming SSE
  events in the same `[host] line` format as `fleet apply`.
- **G3** `mooncake fleet logs --all` attaches to every peer's current run
  simultaneously, multiplexed.
- **G4** `mooncake fleet facts <host>` pretty-prints the peer's facts.
- **G5** Every command exits cleanly on `^C`; remote runs are not affected
  (consistent with spec-43's cancellation story).

**Non-goals:**

- Drift detection. Status reports queue depth and last-run status, not
  whether the peer's state matches a declared plan. Drift is a separate
  story (and an enterprise-epic concern; see `epic-cluster-management.md`).
- Historical run search across the fleet. `fleet logs <host> <run_id>` is
  not in v1 — use `mooncake runs` over SSH if you need that. Could add
  later.
- TUI dashboards. Plain stdout only in v1, consistent with the
  log-multiplexing decision in spec-43.

---

## Reuse map

**Reused:**

- `peers.toml` + transport client (`internal/fleet/transport/agentd.go`)
  from spec-43.
- Multiplexer (`internal/fleet/multiplex.go`) from spec-43 — same writer,
  same color logic; the input source is different but the output format is
  identical.
- agentd endpoints: `GET /v1/version`, `GET /v1/facts`, `GET /v1/runs`,
  `GET /v1/runs/{id}/events`. All exist today; nothing daemon-side
  changes in this spec.

**New:**

| Component | Location |
|---|---|
| `fleet status` table renderer | `cmd/fleet.go` (subcommand) |
| `fleet logs` reattach loop | `cmd/fleet.go` (subcommand) |
| `fleet facts` formatter | `cmd/fleet.go` (subcommand) |
| Helper: fetch per-peer summary | `internal/fleet/inspect.go` |

---

## `fleet status` — wire calls per peer

For each peer in `peers.toml`, in parallel:

1. `GET /v1/version` → `{version, hostname, system_mode, queue_depth, runs_running, uptime_sec}`.
2. `GET /v1/runs?limit=1` → the most recent run record. Status field tells
   us success/failed/running/queued/interrupted.
3. `GET /v1/facts?fields=os,os_version,arch` → only the three OS-shape
   fields, cheap.

Each call has a 3-second timeout. On error, the row shows `unreachable`
with the underlying error in a footnote.

### Rendered table

```text
$ mooncake fleet status
HOST       ADDR                   STATE        OS              MOONCAKE  QUEUE  LAST RUN
laptop     local                  ok           arch 6.6.4      0.9.0     0      success      2m ago
desktop1   desktop1.lan:7878      ok           arch 6.6.4      0.9.0     0      success      4m ago
desktop2   desktop2.lan:7878      running      arch 6.6.4      0.9.0     1      —            in flight
macbook    macbook.lan:7878      ok           darwin 14.4     0.9.0     0      failed       18h ago
vps-1      vps-1:7878             unreachable  —               —         —      —            —

✔ 3 ok, 1 running, 1 failed (last), 1 unreachable
```

- The `local` peer (the controller itself, if it has agentd running on the
  unix socket) is included by default. `--remote-only` excludes it.
- `STATE` summarizes the rest of the row: `ok` (reachable, no queue), `running`
  (a run is in progress), `failed` (last run failed), `unreachable` (network
  error), `stale` (last seen > 10 min ago for cached entries — see below).
- `--json` flag emits per-peer records for scripting.

### Optional: cached/sticky last-seen

By default `fleet status` is real-time — each invocation re-probes peers.
Add a `--cache` flag that writes per-peer status to `~/.cache/mooncake/fleet-status.json`
with the last successful probe, so a follow-up `--from-cache` reads it
without re-probing. Useful for shell prompts / status bars later.

---

## `fleet logs` — semantics

```
mooncake fleet logs <peer>            # attach to latest in-flight run on peer
mooncake fleet logs <peer> <run_id>   # attach to a specific run id
mooncake fleet logs --all             # multiplex latest runs across all peers
mooncake fleet logs --since 5m <peer> # show all events from runs in last 5m
```

### Algorithm for "latest run"

```go
runs := peer.ListRuns(limit=5)
for _, r := range runs {
    if !r.IsTerminal() {
        return r // newest in-flight
    }
}
return runs[0] // most recent (terminal)
```

Then `GET /v1/runs/{id}/events` and stream. If the run is already terminal,
the SSE handler replays JSONL and closes — same behavior as live.

### `--follow` flag

For terminal runs, `--follow` polls every 5 seconds for a *new* in-flight
run on the peer and attaches when one appears. Useful for "watch this peer
until something happens." Default off.

### `--all` algorithm

For each peer in `peers.toml`:
1. Pick latest run (as above).
2. Open `GET /v1/runs/{id}/events`.
3. Feed events into the shared multiplexer (reusing
   `internal/fleet/multiplex.go`).

Exit when *every* peer's run is terminal AND its SSE stream has closed.

---

## `fleet facts` — formatter

```
mooncake fleet facts <peer>            # full facts JSON for peer
mooncake fleet facts <peer> <key>      # one fact (dot-path)
mooncake fleet facts --query <key>     # one fact across ALL peers, tabular
```

`<key>` uses the same dot-path syntax as `mooncake query` does today (see
`internal/mcp/tools.go:HandleFactQuery`). Reuse that logic — extract from
MCP to a shared helper if not already.

Multi-peer query output:

```text
$ mooncake fleet facts --query go_version
HOST       go_version
laptop     1.22.3
desktop1   1.22.3
desktop2   1.21.6
macbook    1.22.3
```

Highlights divergence at a glance, which is the most common reason to ask
this question on a personal fleet.

---

## Tasks

### Task 1 — Per-peer inspector helper

1. New `internal/fleet/inspect.go`:
   - `Probe(ctx, peer Peer) (Status, error)` runs the three GETs in
     parallel with a shared timeout, returns a `Status` struct.
   - `Status` mirrors the table columns; serializable to JSON for the
     `--json` flag.

### Task 2 — `fleet status` subcommand

1. Extend `cmd/fleet.go` with `status` subcommand.
2. Parallel `Probe` across peers (bounded by `--parallel`, default all).
3. Table renderer using `text/tabwriter` for alignment. Color the STATE
   column (green/yellow/red) when stdout is a TTY.
4. `--json` short-circuits the renderer and emits one JSON object per
   line (jsonl).
5. `--cache` / `--from-cache` semantics (deferred to task 6).

### Task 3 — `fleet logs` subcommand

1. `logs <peer>` resolves latest run (or honors explicit run id).
2. Reuse the SSE consumer from spec-43's transport client. Reuse the
   multiplexer for output (even with one peer, the `[host]` prefix is
   useful and the code path is shared).
3. `--all` opens one stream per peer; same multiplexer.
4. `--follow`: after the current stream closes, poll for in-flight every
   5 seconds.

### Task 4 — `fleet facts` subcommand

1. `facts <peer>` calls `GET /v1/facts`, pretty-prints JSON.
2. `facts <peer> <key>` post-filters in Go (existing dot-path helper).
3. `--query <key>` does fan-out + table render.

### Task 5 — Optional caching

1. `~/.cache/mooncake/fleet-status.json` writer/reader.
2. `--cache` writes after success; `--from-cache` reads instead of
   probing.
3. Mark cached rows with a small "(cached)" suffix on `last seen` column.

### Task 6 — Tests

1. `Probe` against a fake agentd (httptest.Server returning canned
   responses for the three endpoints).
2. Status renderer: snapshot-test the rendered table for a fixed
   `[]Status` input.
3. `logs --all` with two fake peers running concurrent SSE streams;
   assert the merged output matches an expected multiset of lines.

---

## Open questions

1. **What counts as "stale"?** A peer that hasn't been probed in N minutes
   under `--from-cache`. Default 10 min; configurable later.
2. **Should the controller itself appear as a peer?** Lean yes — it's a
   useful "is my local agentd up" check. `--remote-only` excludes it.
3. **Do we need `fleet runs --all` (list recent runs across the fleet)?**
   Out of scope for v1; `mooncake runs` on each peer over SSH covers it
   for now. Add later if friction appears.
4. **Color theme.** STATE coloring uses 3 colors today. If we ever support
   themes (light/dark terminals), this needs a palette abstraction.
   Premature; defer.
