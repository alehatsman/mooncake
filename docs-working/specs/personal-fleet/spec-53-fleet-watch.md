# Spec 53: Fleet Watch — live SSE stream across peers

**Epic:** Personal Fleet — see [`epics/done/epic-personal-fleet.md`](../../epics/done/epic-personal-fleet.md).
**Status:** Draft
**Effort:** S (~2 days)
**Value:** Medium — closes the "what is the fleet doing *right now*" gap.
`fleet logs` only reattaches to one peer's latest run; `fleet watch` is
the multi-peer, always-on equivalent. Useful when a drift watcher fires,
when another operator/agent kicks off a run, or when a scheduled job
runs while you're not looking.
**Depends on:** spec-46 (`fleet logs`, transport, multiplexer).

---

## Problem

After spec-46, an operator can:

- `mooncake fleet apply` — start a run and watch it.
- `mooncake fleet logs <peer>` — reattach to a single peer's latest run.
- `mooncake fleet logs --all` — reattach to *every* peer's latest run,
  multiplexed.

`fleet logs --all` looks like the right tool, but it isn't:

1. It picks each peer's most recent run *at command start*. If a peer
   has no in-flight run when you run it, you get the last terminal run's
   replayed events and the stream closes. You don't see anything that
   starts later.
2. When all selected runs reach a terminal state, `fleet logs --all`
   exits. To keep watching, you'd have to re-run it in a loop.
3. There's no way to "subscribe to the fleet" — you have to know about a
   specific run to attach to it.

The gap shows up the moment something else drives the fleet:

- A scheduled run kicks off on `desktop1` at 03:00. You'd like to see it
  the next morning without trawling `fleet logs desktop1 <run_id>`.
- A drift watcher (post-spec-46 follow-up) fires `mooncake apply` on
  `macbook`. You want a live view if you happen to be at the terminal.
- An AI agent or co-operator submits a plan from another controller.
  You'd like to watch over their shoulder without coordinating.

`fleet watch` is the "tail -f for the fleet" command that fills this
gap: one persistent stream that surfaces every event from every peer
until you `^C`.

---

## Goals

- **G1** `mooncake fleet watch` opens a long-lived multiplexed stream
  across every peer in `peers.toml` (subject to `--peers` /
  `--peer-filter`). New runs that start on any peer appear in the
  stream *without* having to restart the command.
- **G2** When a run finishes on a peer, its events stop flowing but the
  command keeps running, ready to pick up the next run on that peer.
- **G3** Output format matches `fleet apply` / `fleet logs`: the
  existing `[peer]` Multiplexer with per-peer color and the existing
  `formatEvent` renderer. Operators recognize the format on sight.
- **G4** `--json` emits one JSON event per line (`{peer, run_id, seq,
  type, timestamp, data}`) for piping into `jq` / dashboards.
- **G5** `^C` cleanly closes every stream and exits 0. Remote runs are
  not affected (consistent with spec-46's "remote runs continue"
  contract).
- **G6** A transport error on one peer does NOT take the command down;
  it surfaces inline as a `*** disconnected ***` line, reconnects with
  backoff, and other peers keep streaming.

**Non-goals:**

- **Historical replay.** v1 only shows events from "now" onward (where
  "now" is defined by the v1 design below — see §Design). Replay of
  events from past terminal runs is `fleet logs <peer> <run_id>`
  territory; a `--since` flag is an open question.
- **Filtering by event type.** Lean: no `--kind step.stdout` filter in
  v1. The multiplexer's prefix + `formatEvent` makes scrolling tolerable
  even at full firehose. Add if asked.
- **TUI dashboard.** Plain stdout only, same as every other
  personal-fleet command.
- **Daemon-side broadcast endpoint.** v1 reuses the existing per-run
  SSE; no new agentd endpoint. See §Design v1 vs v2.
- **Cross-fleet aggregation.** The command's scope is one
  `peers.toml`. If you have multiple controllers / multiple fleet
  configs, run one `fleet watch` per config.

---

## Reuse map

**Reused:**

- `peers.toml` loader + `selectPeers` / `--peer-filter` evaluator from
  `cmd/fleet.go`.
- `transport.Client.Stream` (SSE consumer) and `transport.Client.ListRuns`
  from `internal/fleet/transport/`.
- `fleet.Multiplexer` from `internal/fleet/multiplex.go` — same prefix,
  color, and renderer the rest of the fleet commands use.
- `fleet.ShouldColor` for `NO_COLOR` / TTY detection.
- The `streamOnePeer` cycle in `cmd/fleet_logs.go` is the right shape
  for one iteration; v1 wraps it in a poll loop.

**Extended:**

- `cmd/fleet.go` — register the new `watch` subcommand. Reuse the
  `--peers`, `--peers-file`, `--peer-filter`, `--no-color`, `--json`
  flag idioms already established by `fleet apply` / `fleet logs`.

**New:**

| Component | Location |
|---|---|
| `fleet watch` subcommand | `cmd/fleet_watch.go` (mirrors `fleet_logs.go`) |
| Per-peer poll-and-attach loop | `cmd/fleet_watch.go` (private helper) |
| JSON output sink | `cmd/fleet_watch.go` (small `--json` path that bypasses Multiplexer) |

---

## Design

### v1: controller-side polling, no daemon change

Three architectural options were considered:

1. **(a) Controller polls `GET /v1/runs?status=running` on each peer,
   attaches to each found run via the existing per-run SSE
   (`/v1/runs/{id}/events`). When the SSE closes, drop back to polling
   that peer for the next run.** Zero daemon change. The cost is a
   small polling delay (1–2s) between a run starting on a peer and
   `fleet watch` noticing it; events from before the attach instant on
   that *specific run* are not lost because the per-run SSE replays
   JSONL on subscribe (see `runEventsHandler` in
   `internal/agentd/runs_handler.go`).

2. **(b) A new daemon endpoint `GET /v1/events` that streams ALL events
   for ALL runs (current and future), backed by a new fleet-wide hub on
   top of the existing per-run `Hub`.** Cleaner from the controller's
   point of view (subscribe once, see everything), but it requires a
   second broadcaster, a back-pressure design for multi-run fan-in, and
   a new auth surface. Real cost: ~200 daemon LOC + tests.

3. **(c) `GET /v1/events?since=<ts>` with a replay window backed by an
   on-disk events index across runs.** A superset of (b); strictly more
   work. The only thing it buys over (b) is replay across daemon
   restarts.

**Lean (a) for v1.** It ships in a day, requires no daemon change, and
is sufficient for the personal-fleet scale (1–10 peers). The cost is
the polling delay; in practice this is bounded by `poll_interval`
(default 2s), and the existing per-run SSE replay handles the small
window of events between "run started" and "controller attached" on the
peer that already has the run.

(b) becomes a follow-up if (a)'s polling cost — both the bandwidth
and the missed-event-around-run-boundaries edge case — proves real.
This spec calls it out as a v2 path but does not commit.

### Per-peer state machine

For each peer P in the selected set, run an independent goroutine:

```
state = POLLING
loop:
  switch state:
  case POLLING:
    runs := P.ListRuns(limit=5)
    for r in runs:
      if r.Status in {queued, running} and r.ID not in attached_set[P]:
        attached_set[P].add(r.ID)
        state = ATTACHED(r.ID)
        break
    if state == POLLING:
      sleep poll_interval (default 2s, jittered)
      continue
  case ATTACHED(run_id):
    // Reuses streamOnePeer's body. Emits KindSubmit "attached to run
    // <id>" once on entry; KindEvent for each SSE event; KindDisconnect
    // on stream end without context cancellation.
    err := P.Stream(ctx, run_id, sink)
    state = POLLING
```

`attached_set[P]` prevents re-attaching to the same run if it's still
visible in `ListRuns` after the SSE closed (which is normal — terminal
runs stay listed). The set is per-peer and unbounded for the lifetime
of the command; in a personal-fleet's expected ~hours-to-days session
this stays small. (Open question: cap to last 64 run IDs and evict LRU
if memory ever becomes a concern.)

### Output

Default: multiplexed prefix lines, identical to `fleet logs`. The
first attach to a peer emits the `KindSubmit` "attached to run <id>"
control line, so the operator sees the boundary between runs:

```
$ mooncake fleet watch
fleet watch: 4 peer(s)
[laptop  ] attached to run 1730000001
[laptop  ] ▶ run started
[laptop  ]   ▸ install dev tools
[laptop  ]     ✔ install dev tools
[laptop  ] ✔ run complete success: 1/1 changed, 0 failed, 0 skipped (240ms)
[desktop1] attached to run 1730000005
[desktop1] ▶ run started
[desktop1]   ▸ apply firewall rules
...
^C
fleet watch: stopped.
```

`--json` skips the Multiplexer entirely and emits one JSON line per
event, with the peer and run-id annotated. KindSubmit / KindError /
KindDisconnect surface as objects with a `kind` field:

```json
{"kind":"attached","peer":"laptop","run_id":"1730000001"}
{"kind":"event","peer":"laptop","run_id":"1730000001","seq":1,"type":"run.started","timestamp":"...","data":{}}
{"kind":"event","peer":"laptop","run_id":"1730000001","seq":2,"type":"step.started","timestamp":"...","data":{"name":"install dev tools"}}
{"kind":"disconnected","peer":"desktop1","run_id":"1730000005","reason":"connection reset"}
{"kind":"reattach","peer":"desktop1","backoff_ms":500}
```

### Reconnection

When `P.Stream` returns a non-context error (transport reset, daemon
restart, network blip), the per-peer goroutine:

1. Emits `KindDisconnect` so the user sees the gap inline.
2. Sleeps with exponential backoff (500ms → 1s → 2s → 4s → cap at
   8s), jittered by ±25%.
3. Returns to POLLING. The next `ListRuns` call either finds the same
   run still in-flight (we re-attach; the per-run SSE replays from
   JSONL so we don't miss events that landed while we were
   disconnected) or finds nothing (we resume polling).

A peer whose first `ListRuns` fails (token wrong, daemon down at
startup) emits `KindError` once and then enters the backoff loop. The
command does NOT exit on per-peer setup failure; an unreachable peer is
a normal steady state for personal fleets where one machine is asleep.

### Flags

```
--peers <a,b,c>     # name filter, same semantics as fleet apply
--peers-file <path> # override ~/.config/mooncake/peers.toml
--peer-filter k=v   # tag/name/os/role filter, AND-then-OR
                    # (reuses cmd/fleet.go:parseFilterFlags +
                    # validatePeerFilterKeys)
--json              # JSONL output, see above
--no-color          # disable ANSI in the [peer] prefix
--poll-interval     # default 2s; how often a POLLING goroutine asks
                    # the peer for new in-flight runs
```

Explicitly NOT in v1: `--since <duration>`, `--kind <event-type>`,
`--run <run-id>` (single-run subscribe is what `fleet logs <peer>
<run_id>` already does), `--include-terminal` (replay last terminal run
on attach).

### Stdin / TTY

Read-only command. No prompts, no interaction. Same `^C` semantics as
`fleet logs`: first signal cancels the context and lets the streams
drain cleanly; second signal hard-exits 130.

### Default scope: all peers

`fleet watch` with no flags watches every peer in `peers.toml`. Lean
toward "all by default" because the command's value proposition is
"see *whatever* the fleet is doing"; requiring `--peers` would
contradict that. `--peers` / `--peer-filter` narrow the set.

### Heartbeats / quiet mode

When nothing is happening (all peers in POLLING, no in-flight runs),
`fleet watch` is silent. Don't emit "..." every N seconds — operators
will leave this running in a terminal pane and visual noise will train
them to dismiss it. The lack of output IS the signal: "nothing is
running." If the operator wants to confirm the command itself is
alive, `^C` and re-run is the muscle memory; we don't need a heartbeat
for that.

(Open question: optional `--heartbeat 30s` for operators who do want a
"still watching" pulse.)

---

## Implementation outline

1. **`cmd/fleet_watch.go`** — new file, mirrors `cmd/fleet_logs.go`'s
   structure. Define `fleetWatchCommand()` returning the cli.Command;
   register it in `cmd/fleet.go` next to `fleetLogsCommand()`.
2. **Flag parsing & peer selection.** Copy the shape from
   `fleetLogsAction`: load peers, apply `selectPeers`, apply
   `--peer-filter` via the existing `parseFilterFlags` /
   `peerMatchesFilters` helpers. Reject if 0 peers match.
3. **`watchPeers` driver.** Build a `Multiplexer` (or a `--json`
   writer), spawn one goroutine per peer running the state machine in
   §Per-peer state machine, fan into a shared `chan fleet.PeerEvent`,
   `mux.Drain` on the consumer side. Signal handling mirrors
   `streamPeers` in `cmd/fleet_logs.go` exactly.
4. **`watchOnePeer` goroutine.** The per-peer poll + attach loop.
   Reuses `transport.Client.ListRuns` for the poll and the existing
   `streamOnePeer`-style cycle for the attach. New: the `attached_set`
   map and the backoff helper.
5. **`--json` sink.** A small alternative to the Multiplexer that
   marshals `PeerEvent` to a JSON line. Attach / disconnect / reattach
   are emitted as `{"kind":...}` objects; KindEvent expands the embedded
   `transport.Event`. No color, no padding.
6. **Tests** (`cmd/fleet_watch_test.go`):
   - Two `httptest.Server` peers with canned `/v1/runs` + SSE
     responses. Assert the multiplexed output contains events from
     both peers in any order and emits the right KindSubmit / SSE
     event lines.
   - Reattach test: a peer that returns an in-flight run, closes the
     SSE mid-stream, then returns a *new* in-flight run on the next
     poll. Assert both runs' events appear in order and the second
     attach emits a KindSubmit boundary line.
   - Backoff test: a peer whose `ListRuns` fails for the first two
     polls then succeeds; assert KindError appears once and the third
     poll attaches.
   - `--json` test: assert one parseable JSON object per output line
     across an attached run.
   - `^C` test: cancel the context; assert all goroutines exit and the
     final banner prints.

---

## Open questions

1. **Default scope — all peers vs explicit `--peers`?** Lean "all by
   default" for the reasons in §Default scope. An operator with a
   `peers.toml` of 8 peers and one always-asleep VPS gets a noisy
   `KindError` from the VPS on startup; the backoff loop quiets it
   thereafter. Acceptable, but worth checking on real fleets.
2. **Poll interval default.** 2 seconds feels right for personal-fleet
   scale. Too aggressive: wastes a `/v1/runs` round-trip per peer per
   second on idle fleets. Too slow: noticeable lag between a run
   starting on a peer and `fleet watch` picking it up. The interval is
   exposed as `--poll-interval`; default 2s is the bet.
3. **`--since <duration>`.** With v1's architecture (a), `--since` is
   awkward: we can ask each peer for its recent run list and attach to
   any run that started within the window, but the events on terminal
   runs will *replay* their full JSONL on attach, so `--since 5m` on a
   peer with a 30-minute run that started 6 minutes ago would dump 30
   minutes of events. Cleaner with v2 (a `/v1/events?since=` endpoint).
   Defer.
4. **Should `--json` include the daemon's raw SSE payload bytes or the
   decoded `transport.Event`?** Lean decoded — operators piping to `jq`
   want a stable schema. The trade-off is that we lose
   forward-compatibility for new event types the controller doesn't
   recognize. Mitigation: the decoded shape preserves `data` as
   `json.RawMessage`, so unknown fields survive.
5. **v2 daemon endpoint `GET /v1/events`.** Worth the daemon-side lift?
   Strictly an optimization over (a) for personal-fleet scale; becomes
   compelling at 50+ peers (one persistent SSE per peer instead of N
   polls + M attaches). Defer until someone hits the bandwidth ceiling.
6. **Heartbeat / `--heartbeat`.** A "still watching" pulse every N
   seconds when no events flow. Defer; ship silent.
7. **Mixed-transport peers.** A `peers.toml` entry with
   `transport=ssh-bootstrap` (from spec-44) has no agentd to subscribe
   to. v1: silently skip non-agentd peers (with a one-line warning to
   stderr at startup), mirroring `fleet logs --all`.
8. **Local peer (controller's own agentd, if running).** Should
   `fleet watch` include the controller's local agentd by default? Lean
   yes for symmetry with `fleet status`. If absent from `peers.toml`,
   it's a no-op; if present, it's another peer.

---

## Success criteria

After this spec lands:

1. `mooncake fleet watch` running in a terminal pane surfaces events
   from any run started on any peer in `peers.toml`, without restart.
2. `^C` exits cleanly; remote runs continue.
3. A peer that's unreachable at start, comes online mid-session, and
   then starts a run shows up in the stream without operator
   intervention.
4. `mooncake fleet watch --json | jq 'select(.type=="step.failed")'`
   gives the operator a "live failure feed" suitable for piping into
   a notifier.
5. The implementation reuses `transport.Client.Stream`,
   `fleet.Multiplexer`, and the `--peers` / `--peer-filter` evaluator
   already shipped — no daemon change.
