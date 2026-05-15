# Spec 54: Fleet PS — list in-flight runs across peers

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md).
**Status:** Draft
**Effort:** XS (~1 day)
**Value:** Small but daily — answers "is anyone applying anything to my
fleet right now?" and "which peer is taking so long?" without ssh'ing
into each box. Pure controller-side fan-out + table renderer; zero
daemon work.
**Depends on:** spec-46 (probe transport, table renderer pattern, peer
selection), spec-50 (extended `--peer-filter` keys).

---

## Problem

After spec-46 ships, `fleet status` gives an at-a-glance health table
with one row per peer and a single boolean `RUNNING` column. That's
fine for "is everything green?" but not for "*what* is each peer
running?":

- `RUNNING yes` doesn't tell you which run ID, which plan, or how long
  it has been going.
- `QUEUE 0 (+1 running)` doesn't tell you which run.
- Two operators (or an MCP agent and a human) applying to the fleet
  concurrently is now plausible — `fleet status` will say "one running"
  on each box but won't show whether it's the same run the human kicked
  off, or someone else's.

Today the workaround is `ssh peer 'mooncake runs --status running'`
per peer, or repeated `mooncake fleet logs <peer>` attaches. Neither
is a one-shot enumeration.

agentd already persists runs (`internal/agentd/store.go:Run`) and
exposes `GET /v1/runs?status=running&limit=N`. The transport client
already has `Client.ListRuns(ctx, limit int)` (no status filter today —
adds one).  This spec is fan-out + tabwriter, mirroring
`fleet status`.

---

## Goals

- **G1** `mooncake fleet ps` fans out to every peer in `peers.toml`,
  calls `GET /v1/runs?status=running` in parallel, and renders a
  one-line-per-run table.
- **G2** `--all` widens the query to include recently-completed runs
  (`?limit=N` without a status filter, newest-first). Default `N=5`
  per peer.
- **G3** `--status <s>[,s,...]` filters by run status
  (`running`, `queued`, `success`, `failed`, `interrupted`). Repeats
  the daemon-side query when one value is given; client-side filter
  when multiple. Default: `running` (with `--all`: no status filter).
- **G4** Standard fleet filters apply: `--peers`, `--peer-filter`
  (extended keys per spec-50), `--peers-file`.
- **G5** `--json` emits JSONL — one object per run, with the peer
  name + the `RunRecord` shape from `transport/agentd.go`.
- **G6** Unreachable peers are listed in a footnote (same shape as
  `fleet status`), not as table rows. Exit code mirrors `ps`: 0 even
  when there are no in-flight runs, non-zero only on transport
  failure (per `statusExitCode`-style aggregation).

**Non-goals:**

- `--watch` / `-w`. Out of scope for v1. Users wrap with `watch -n 2
  mooncake fleet ps`. (If demand appears, build on the spec-46
  multiplexer the way `fleet watch` would — but `fleet watch` itself
  is a separate spec in the qol-features brainstorm.)
- Cross-peer aggregation (e.g. "5 running across 3 peers, longest
  18m"). Add a summary line, not a roll-up table.
- Killing / cancelling runs from `ps`. Read-only by design. Killing
  is a separate `fleet cancel <peer> <run_id>` story.
- Filter by plan path, by tag, by goal. Trivial post-filter once the
  data is in hand, but defer to a follow-up if anyone asks.
- Reporting the *current step name* of each running run. The Run
  record doesn't carry it (see Open Question 1).

---

## Reuse map

**Reused:**

- `peers.toml` loader + `selectPeers` + `parseFilterFlags` +
  `peerMatchesFilters` from `cmd/fleet.go` (spec-46 + spec-50).
- `fleet.ProbeAll` shape — the fan-out pattern, the parallel cap, the
  per-peer error surfacing. `fleet ps` is a sibling fan-out call that
  hits one endpoint per peer instead of three.
- `transport.Client.ListRuns(ctx, limit int)` from
  `internal/fleet/transport/agentd.go` — already exists; extend to
  accept a `status` parameter and propagate the `before` cursor.
- `fleet.ShouldColor` + the tabwriter idiom from
  `cmd/fleet_status.go:renderStatusTable`.
- `dash()`, `oneLineErr()` helpers from `cmd/fleet_status.go`.
- `humanDuration` from `internal/fleet/inspect.go` for the AGE
  column.

**New:**

| Component | Location |
|---|---|
| `fleet ps` subcommand | `cmd/fleet_ps.go` (new) |
| Per-peer run fetcher (parallel + capped) | `internal/fleet/inspect.go` (add `FetchRuns`) |
| `ListRuns` accepts a status filter | `internal/fleet/transport/agentd.go` (extend signature) |
| Row-shaped `RunRow{Peer, Run}` for the table | `internal/fleet/inspect.go` |

---

## Design

### Wire calls per peer

For each selected peer, in parallel (capped by `--parallel`, default
unbounded — matches `fleet status`):

```
GET /v1/runs?status=running&limit=N        (default)
GET /v1/runs?limit=N                       (--all)
GET /v1/runs?status=<s>&limit=N            (single --status)
```

Multi-value `--status running,queued` does one call per status per
peer and merges results client-side. Cheap enough; the daemon doesn't
support comma-separated status values today and adding that is more
scope than `fleet ps` needs.

Timeout: same default as `fleet status` (3s per peer, `--timeout`
flag).

### Rendered table

```text
$ mooncake fleet ps
HOST       RUN_ID                      STATUS    AGE    PLAN
desktop2   01HXG4...J4Z3K7              running   12m    machines/desktop2/index.yml
macbook    01HXG4...P8N2R1              running   3m     plans/dotfiles.yml

✔ 2 running across 2 peers (3 total accessible)
```

`--all`:

```text
$ mooncake fleet ps --all
HOST       RUN_ID                      STATUS        AGE    PLAN
desktop2   01HXG4...J4Z3K7              running       12m    machines/desktop2/index.yml
laptop     01HXG3...A1B2C3              success       2m     machines/laptop/index.yml
laptop     01HXG2...D4E5F6              success       18m    machines/laptop/index.yml
macbook    01HXG4...P8N2R1              running       3m     plans/dotfiles.yml
macbook    01HXG3...G7H8J9              failed        1h     plans/dotfiles.yml
desktop1   01HXG3...K0L1M2              interrupted   2h     machines/desktop1/index.yml

✔ 6 runs across 4 peers (2 running, 2 success, 1 failed, 1 interrupted)
```

Columns (essential set, narrow on purpose):

| Column | Source | Notes |
|---|---|---|
| `HOST` | `peers.toml` name | grouping anchor; stable across runs |
| `RUN_ID` | `Run.ID` | ULID; full width by default. `--short` truncates to last 10 chars |
| `STATUS` | `Run.Status` | color: yellow running, green success, red failed/interrupted, dim queued |
| `AGE` | `humanDuration(now - pickTime)` | `pickTime` = StartedAt for running; FinishedAt for terminal; QueuedAt for queued |
| `PLAN` | `Run.PlanPath` | relative to peer's daemon-side fs; long. Truncated middle if > 60 chars (TTY only) |

Deferred columns (open questions, not v1):

- `CURRENT_STEP` — would need a daemon change to expose. See OQ1.
- `GOAL` — `Run.Goal` is usually empty in `fleet apply` flows;
  surface via `--json` instead.
- `QUEUED_AT` / `STARTED_AT` — absolute timestamps. `--json` carries
  them; the table sticks to `AGE` to stay narrow.

### Sort order

Default: grouped by peer (preserve `peers.toml` order), then within
peer by `StartedAt` descending (newest first). Rationale: when a user
asks "what's `desktop2` doing?", they scan to the `desktop2` block;
they don't want runs interleaved across peers by global timestamp.

`--sort=age` flips to globally sorted by AGE ascending (oldest first
— "which peer is taking so long?" use case).

### Empty result

```text
$ mooncake fleet ps
no in-flight runs (4 peers accessible, 0 unreachable)
```

Don't print an empty table. Don't exit non-zero — empty is the
common case, mirrors `ps` muscle memory.

### Unreachable peers

Footnote, after the table, same shape as `fleet status`:

```text
  vps-1: dial tcp vps-1:7878: connect: connection refused
```

Exit code: 0 if at least one peer responded; non-zero only when
*every* peer is unreachable (transport failure dominates). Diverges
from `fleet status`'s 0/1/2 mix because `ps` is read-only and
unreachable peers don't mean "the fleet is broken" — they mean "I
couldn't ask one peer." A warning-but-zero-exit matches `ps`.

### `--json` output

JSONL, one object per run:

```json
{"peer":"desktop2","run":{"id":"01HX...","status":"running","plan_path":"...","queued_at":"...","started_at":"...","finished_at":""}}
```

Per-peer probe errors emit a sentinel record with `"error"` set:

```json
{"peer":"vps-1","error":"dial tcp vps-1:7878: connect: connection refused"}
```

So a `jq` consumer sees the unreachable peer without losing the run
records from the reachable ones.

---

## Implementation outline

### Phase A — `ListRuns` status parameter (transport-side)

1. Extend `transport.Client.ListRuns(ctx, opts ListRunsOpts)` to
   accept `{Status, Limit, Before}`. Keep the old `ListRuns(ctx,
   limit)` signature as a thin wrapper for spec-46's call site.
2. The daemon already accepts `?status=&limit=&before=` on
   `GET /v1/runs` (`internal/agentd/runs_handler.go:listRunsHandler`).
   No daemon change.

### Phase B — Per-peer fetcher in `internal/fleet/inspect.go`

1. Add `FetchRuns(ctx, peer Peer, opts FetchOpts) ([]RunRecord,
   error)` — bounded by `opts.Timeout`, returns the raw records.
2. Add `FetchRunsAll(ctx, peers []Peer, opts FetchOpts) []PeerRuns`
   — parallel fan-out, capped by `opts.MaxParallel`, mirrors the
   `ProbeAll` shape. `PeerRuns{Name, Runs, Error}` so unreachable
   peers surface alongside the data instead of dropping.
3. Reuse `humanDuration` for the rendered AGE.

### Phase C — `fleet ps` subcommand

1. New `cmd/fleet_ps.go` with `fleetPsCommand()`:
   - Flags: `--peers`, `--peers-file`, `--peer-filter` (extended keys),
     `--parallel`, `--timeout`, `--status`, `--all`, `--json`,
     `--sort`, `--short`, `--no-color`.
   - Wire into `fleetCommand` alongside `status`, `apply`, `logs`, etc.
2. Selection: same `selectPeers` + `parseFilterFlags` +
   `peerMatchesFilters` path as `fleet apply`. Unknown peers warn
   (same as `fleet status`).
3. Call `FetchRunsAll` with `Status` derived from `--status` (default
   `"running"`; `--all` clears it; multi-value triggers the
   one-call-per-status loop).
4. Render: TTY → tabwriter with color; `--json` → JSONL.
5. Empty result: print the "no in-flight runs" line, exit 0.

### Phase D — Tests

1. `FetchRunsAll` against two `httptest.Server` fakes — one returning
   two running runs, one returning a 500. Assert merged result has
   the running runs + the error entry, both ordered by input.
2. Snapshot-test `renderPsTable` for fixed `[]PeerRuns` input
   covering: empty result, mixed running+terminal under `--all`, one
   unreachable peer.
3. `--status running,queued` issues two HTTP calls per peer (httptest
   counts requests).
4. End-to-end: `cmd/fleet_ps_test.go` runs the action against a
   fake-agentd test fixture, asserts the exit code, JSONL shape,
   and that `--peer-filter os=darwin` actually narrows the fan-out.

---

## Open questions

1. **Current-step column.** The `Run` record persists `Status` and
   timestamps but *not* the currently-executing step. To populate
   `CURRENT_STEP`, the daemon would need to track the latest
   `step.started` event id per running run, exposed either on
   `GET /v1/runs/{id}` or as a new `current_step` field on the Run
   record. Worth doing? Lean **no for v1** — `fleet ps` is a glance
   command; "which step?" is what `fleet logs <peer>` is for. If
   demand appears, add `current_step` to the Run record in a
   follow-up (cheap; the worker already sees the event).

2. **Default scope: `running` only, or `running` + `queued`?**
   `running` matches `ps` muscle memory and is the most common
   query. `queued` runs *also* answer "is anyone applying right
   now?" (the apply hasn't started yet but it's coming). Lean
   **`running` by default**; document that `--status running,queued`
   gives the full "in-flight" picture. The qol-features brainstorm
   says "in-flight runs" — ambiguous on this point.

3. **`--limit N` per peer or global?** With `--all`, do you want
   "newest 5 per peer" (per-peer fairness) or "newest 5 across the
   fleet" (global timeline)? Lean **per-peer** — matches the default
   sort order (grouped by peer), and a global cap with N=5 across a
   10-peer fleet would silently hide most peers. Add `--global-limit`
   if anyone asks.

4. **Should the local controller's agentd appear as a peer?**
   `fleet status` includes it (per spec-46 open question 2);
   `fleet apply` doesn't because the controller already has
   `mooncake apply`. For `fleet ps` the question is: does my
   local `mooncake apply` show up in `fleet ps`? Lean **yes** —
   the controller's runs are part of "is anyone applying right now"
   from another operator's perspective, and the daemon's run store
   doesn't distinguish local-submit from controller-submit. Treat
   the controller-host as a peer if its agentd is reachable.

5. **`PLAN` column truncation rule.** Paths like
   `/home/aleh/.local/state/mooncake/synced/desktop2/machines/desktop2/index.yml`
   are common after spec-46's sync layout. Middle-truncate
   (`/.../machines/desktop2/index.yml`)? Trim the synced-root prefix
   (`machines/desktop2/index.yml`)? Lean **trim the synced-root
   prefix** when it matches a known pattern, then middle-truncate the
   remainder if still > 60 chars. Spec the exact rule in the
   subcommand help text.

6. **Watch mode.** Explicitly out of scope, but if someone wants
   `--watch 2s` later, the implementation is one `time.Ticker`
   wrapping the existing action and a clear-screen between renders.
   Don't pre-build it; users who really want it use `watch -n 2`.

---

## Success criteria

After this spec lands:

1. `mooncake fleet ps` prints one row per in-flight run across the
   fleet, grouped by peer, in under a second on a 5-peer LAN.
2. `mooncake fleet ps --all` shows the last 5 runs per peer regardless
   of status, with the same column shape.
3. `mooncake fleet ps --peer-filter os=darwin` narrows the fan-out
   to darwin peers only (proves spec-50 extended keys are reused).
4. `mooncake fleet ps --json | jq '.run.id'` extracts every run id
   without parsing the table.
5. Unreachable peers appear as a footnote, not as table rows, and
   the exit code is 0 unless *every* peer was unreachable.
6. Empty result prints "no in-flight runs" (not a header-only table)
   and exits 0.
