# Spec 55: Fleet Doctor — health check across peers

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md).
Brainstormed in `docs-working/clustermanagement/qol-features.md` §"fleet doctor".
**Status:** Draft
**Effort:** S (~3 days)
**Value:** Medium — answers "is my fleet healthy?" without SSH-ing into each box.
The kernel's doctor already covers the checks; today the friction of running
it on every peer means it doesn't get run. Fanning it out lifts the existing
~16 checks to the fleet without inventing any new check logic.
**Depends on:** spec-43 (peer transport + `peers.toml`), spec-46 (fleet
subcommand pattern + accessibility/exit-code shape), and the kernel doctor
package (`internal/doctor/`).

---

## Problem

`mooncake doctor` runs ~16 checks per peer covering install, system, state,
presets, tools, project, and services. The output already tells the operator
"this peer is on a stale agentd version", "this peer's systemd is misconfigured",
"presets path is broken on this peer" — but only for the *one* box the
operator is sitting at.

To answer "is my fleet healthy?" today, the operator has to:

1. `ssh` into each peer.
2. Run `mooncake doctor`.
3. Eyeball the output.
4. Repeat for every box.

This friction means nobody runs it. Drift accumulates silently on the box you
*didn't* check this month, and the first signal is a failing `fleet apply`
six weeks later.

The kernel already has the check engine. agentd already has the local
context every check needs (the binary, the home dir, the preset paths, the
running daemon). All that's missing is a way to (a) ask each peer to run
its doctor and (b) aggregate the results into one scannable view.

---

## Goals

- **G1** `mooncake fleet doctor` runs the kernel's doctor checks on every
  selected peer in parallel and renders a one-line-per-peer summary table
  (HOST, PASS, WARN, FAIL, ACCESSIBLE).
- **G2** `mooncake fleet doctor --peer <name>` prints the full per-check
  report for one peer — the same shape `mooncake doctor` produces today.
- **G3** `mooncake fleet doctor --json` emits one JSON object per peer
  (JSONL) carrying the peer's name + the full check array, suitable for
  `jq` / CI consumption.
- **G4** A peer running an older agentd that doesn't expose the doctor
  endpoint degrades gracefully — its row shows "doctor unavailable
  (agentd predates spec-55)" and is excluded from the pass/warn/fail
  counts. The command does not fail.
- **G5** Exit code aggregates across peers: 0 if every accessible peer's
  checks are OK, 1 if any peer has a warning or any peer is unreachable,
  2 if any peer has an error. Matches `fleet status`'s 0/1/2 shape so
  scripts can treat the two commands uniformly.

**Non-goals:**

- **Inventing new checks.** This spec exposes the existing kernel doctor
  catalogue across peers. If a check is missing, file a separate spec
  against `internal/doctor/`. The one exception is daemon-internal health
  surfacing (queue depth, recent run errors) — see "Open questions §11".
- **Drift detection.** "Does peer X match a declared plan?" is the
  enterprise-epic drift story (`epic-cluster-management.md`); doctor is
  about local health, not plan conformance.
- **SSH fallback for non-agentd peers.** v1 only fans out to peers with
  `transport = "agentd"`. SSH-bootstrap-only peers are skipped with a
  warning (consistent with `fleet apply`'s behavior). See "Design §
  Transport choice".
- **Continuous monitoring / a watch mode.** One-shot only. Polling is a
  shell loop's job, not the CLI's.
- **TUI / dashboard.** Plain stdout, same as the rest of the fleet
  subcommands.

---

## Reuse map

**Reused:**

- `internal/doctor/` — the check catalogue, `Run`, `Report`, `Result`,
  `Status`, `RenderJSON`, `RenderText`. The fleet-doctor controller and
  daemon paths both call into this package; the wire shape is the existing
  `Report` JSON contract verbatim.
- `peers.toml` + `transport.Client` from spec-43. New `Doctor()` method on
  the client follows the `GetVersion`/`GetFacts` pattern.
- `selectPeers` + `parsePeerFilterGroups` + `peerMatchesFilters` from
  `cmd/fleet.go`. `--peers` / `--peer-filter` semantics are identical to
  every other fleet subcommand.
- Capability-detection pattern from spec-50 (`os=` predicate gracefully
  degrades on daemons that predate the field). HTTP 404 on the new
  endpoint signals "peer is old"; the controller emits a warning and
  excludes the row.
- The fleet status table renderer pattern (`text/tabwriter`, colored
  ACCESSIBLE column, tail summary) from `cmd/fleet_status.go`.

**New:**

| Component | Location |
|---|---|
| Daemon doctor endpoint | `internal/agentd/handlers.go` (`doctorHandler`) |
| Route registration | `internal/agentd/server.go` (`mux.HandleFunc` line near `/v1/version`) |
| Client method | `internal/fleet/transport/agentd.go` (`Client.Doctor`) |
| Per-peer fan-out helper | `internal/fleet/doctor.go` (`RunAll(ctx, peers, opts)`) |
| `fleet doctor` CLI command | `cmd/fleet_doctor.go` (new file, registered in `cmd/fleet.go`) |

---

## Design

### Transport choice — daemon-side, not SSH

The brainstorm lists two architectures: (a) agentd exposes a `/v1/doctor`
that runs the checks locally; the controller fans out and aggregates;
(b) controller SSHes in and shells out to `mooncake doctor --format json`
on each peer.

**v1 is (a).** Justifications:

- agentd is already running on every peer the operator pairs into the
  fleet. SSH is not — and the operator may have deliberately
  pair-without-bootstrap'd (spec-44) precisely to avoid SSH credentials
  for the controller box.
- agentd has all the local context the kernel doctor needs (the binary it
  was launched from, the daemon's home dir, the running services). Asking
  the daemon "doctor yourself" is a one-liner; spawning a `mooncake
  doctor` subprocess from agentd would re-discover the same paths.
- Authentication is solved — bearer tokens already gate every other
  endpoint. SSH would require a second credential.
- Consistent with `fleet status` / `fleet facts` / `fleet logs`, all of
  which use agentd transport and skip non-agentd peers with a warning.

The SSH fallback (b) is theoretically attractive for SSH-bootstrap-only
peers, but those peers don't have an agentd to be unhealthy in the first
place — there's nothing to doctor. If we later add an `ssh-only` mode for
unbootstrapped boxes, revisit.

### Wire shape: `GET /v1/doctor`

```
GET /v1/doctor?section=install,services&strict=true
Authorization: Bearer <token>
```

Query params mirror the existing CLI flags:

- `section=<s>[,<s>]` — repeated section filter, same allowlist as
  `mooncake doctor --section`. Empty = all sections.
- `strict=true|false` — passes through to `doctor.Options.Strict`. The
  daemon doesn't itself act on the resulting exit code; this is so the
  rendered `Report` matches what `mooncake doctor --strict` would show
  on the peer. (Most callers leave it false and let the controller
  aggregate.)
- `skip_project=true|false` — agentd is never invoked from inside a user
  project, so the project checks are always inappropriate. Default
  `true` daemon-side (override possible via the query param, but expect
  noise).

The response body is exactly the `doctor.Report` struct's existing JSON
shape — same field tags, same `checks` array, same `ok / warnings /
errors / infos` counts. No new schema; the daemon calls `doctor.Run` and
returns the `Report` verbatim:

```json
{
  "cwd": "/var/lib/mooncake/agentd",
  "ok": 12,
  "warnings": 1,
  "errors": 0,
  "infos": 3,
  "checks": [
    {"section": "install", "name": "binary", "status": "ok", "message": "mooncake 0.9.0"},
    {"section": "services", "name": "agentd", "status": "warning",
     "message": "queue depth elevated", "fix": "investigate stuck runs"}
  ],
  "started_at": "2026-05-15T14:22:01Z",
  "duration_ms": 184
}
```

The controller can therefore render an individual peer's report by
calling `doctor.RenderText(out, report, noColor)` on the controller side,
feeding it the daemon's `Report` — single source of truth for the
text format.

### Daemon-side check selection

The kernel catalogue includes a `project` section that checks `cwd` for
a `mooncake.yml`. agentd's cwd is its synced root (not a user project),
so the project section would always emit noise. The daemon-side handler
defaults `skip_project=true` regardless of the query param's absence —
explicit `skip_project=false` is honored for debugging.

The other sections all apply meaningfully to the daemon's host (install,
system, state, presets, tools, services). No daemon-side filtering
beyond that.

### Capability detection / graceful fallback

A peer running agentd from before this spec lands won't have `/v1/doctor`.
The handler isn't there; the default `notFoundHandler` returns HTTP 404.
The controller treats 404 from `/v1/doctor` as "doctor unavailable on
this peer" and emits an explicit warning row in the table:

```
HOST       ACCESSIBLE  PASS  WARN  FAIL  NOTE
laptop     yes         12    1     0     —
desktop1   yes         13    0     0     —
macbook    yes         —     —     —     doctor unavailable (agentd predates spec-55)
vps-1      no          —     —     —     unreachable: dial tcp 10.0.0.5:7878: i/o timeout
```

Non-404 errors (token rejected, timeout, malformed response) follow the
existing `fleet status` pattern: the row shows ACCESSIBLE=no with the
error in a footnote under the table.

### CLI surface

```
mooncake fleet doctor                              # aggregated table across all peers
mooncake fleet doctor --peer macbook               # full per-check report for one peer
mooncake fleet doctor --json                       # JSONL: one {peer, report} per line
mooncake fleet doctor --peers laptop,desktop1      # name filter (same as fleet apply)
mooncake fleet doctor --peer-filter tag=workstation  # tag/os/role filter (spec-50)
mooncake fleet doctor --section install,services   # forward to daemon
mooncake fleet doctor --detail                     # full per-check report for *every* peer
mooncake fleet doctor --strict                     # warnings → exit 1
mooncake fleet doctor --timeout 5s                 # per-peer probe timeout
mooncake fleet doctor --parallel 4                 # bounded fan-out
mooncake fleet doctor --no-color
```

`--peer` and `--peers` are mutually exclusive (the singular form is for
drill-down; the plural is a filter). When `--peer <name>` is set, the
output is the per-peer renderer, not the aggregated table.

`--section` is repeatable like `mooncake doctor --section`, accepting the
same allowlist of section names. Aliased to the brainstorm's suggested
`--checks` only if it makes the help text clearer (lean: stick with
`--section` to match the local doctor).

### Aggregated table

```
$ mooncake fleet doctor
HOST       ACCESSIBLE  PASS  WARN  FAIL  LAST RUN
laptop     yes         12    1     0     success 2m ago
desktop1   yes         13    0     0     success 4m ago
desktop2   yes         11    2     0     success 11m ago
macbook    yes         13    0     0     success 18h ago
vps-1      no          —     —     —     —

✔ 4/5 accessible, 3 warnings across 2 peers, 0 failures
  desktop2: warning: presets/path: ~/.mooncake/presets is empty
  desktop2: warning: tools/fzf: not found (used by interactive selector)
  laptop:   warning: services/agentd: queue depth 4 (elevated)
  vps-1:    unreachable: dial tcp 10.0.0.5:7878: i/o timeout

✗ exit 1 (warnings present, --strict not set → exit 0 in non-strict mode)
```

The tail summary lists every warning + failure in `host: section/name: message`
form so the operator doesn't have to drill into `--peer <x>` for the
common "what changed since last week" question. Errors and warnings are
shown inline; OK rows are summarised only by count.

### Per-peer drill-down

`mooncake fleet doctor --peer desktop2` renders the *same* output
`mooncake doctor` would render locally on desktop2 — reuses
`doctor.RenderText` on the controller side fed by the wire `Report`.
The only added line is a single banner identifying the peer:

```
$ mooncake fleet doctor --peer desktop2
peer desktop2 (desktop2.lan:7878) — doctor report:

  install
    binary                 ok     mooncake 0.9.0
    go_runtime             ok     go1.22.3
  system
    facts                  ok
  state
    home_dir               ok     ~/.mooncake (0 RW)
    runs_log               ok     ~/.mooncake/runs.log (12 entries)
    disk_space             ok     342 GB free
  presets
    preset_paths           warn   ~/.mooncake/presets is empty
                                  → run `mooncake init` or set MOONCAKE_PRESETS_PATH
  tools
    git                    ok     2.42.0
    fzf                    warn   not found
                                  → brew install fzf (or apt install fzf)
    sudo                   ok
  services
    agentd                 ok     listening on :7878
    mcp                    ok

11 ok, 2 warnings, 0 errors, 0 infos (duration: 198ms)
```

### JSON output

`--json` emits one JSON object per peer (JSONL), suitable for
`mooncake fleet doctor --json | jq`:

```json
{"peer":"laptop","accessible":true,"unavailable":false,"report":{"ok":12,"warnings":1,"errors":0,"infos":3,"checks":[...]}}
{"peer":"vps-1","accessible":false,"unavailable":false,"error":"dial tcp 10.0.0.5:7878: i/o timeout"}
{"peer":"macbook","accessible":true,"unavailable":true,"error":"HTTP 404: agentd predates spec-55"}
```

Top-level keys are stable (`peer`, `accessible`, `unavailable`, `report`,
`error`); the embedded `report` is the existing `doctor.Report` JSON
contract from `mooncake doctor --format json`.

### Exit code

Follows the `fleet status` shape (`statusExitCode` in
`cmd/fleet_status.go`), adapted:

- **0** — every accessible peer's report has zero warnings AND zero
  errors. Unavailable peers (older agentd) count as warnings-equivalent;
  unreachable peers count as failures-equivalent.
- **1** — at least one peer reports warnings, OR at least one peer is
  unavailable (older agentd), OR `--strict` is set and any
  non-OK status appears. Unreachable peers also fall here.
- **2** — at least one peer reports errors.

The "unreachable → exit 1" choice differs from `fleet status` (which uses
exit 2 for unreachable). Rationale: for *doctor*, an unreachable peer is
itself a finding, not a fatal command failure. Open question §1
challenges this.

### Concurrency + timeouts

Same shape as `fleet status`: `ProbeAll(ctx, peers, timeout, parallel)`
fans out, bounded by `--parallel` (0 = unbounded). Per-peer timeout
defaults to 5s — generous compared to status's 3s because each doctor
runs ~16 checks daemon-side and may stat disks / probe services. The
daemon's own per-check timeout is already 200ms
(`doctor.PerCheckTimeout`), so a full run completes well under the
controller's 5s budget on a healthy peer.

### Authorization

The new `/v1/doctor` endpoint sits behind the same bearer-auth middleware
as everything else. No additional ACL. Operators who can submit a run can
ask for a doctor report; that's symmetric.

---

## Implementation outline

1. **Daemon endpoint.** Add `doctorHandler(w, r)` to
   `internal/agentd/handlers.go`. Parse `section`, `strict`,
   `skip_project` query params. Default `skip_project=true`. Call
   `doctor.Run(opts)` with `opts.Out = io.Discard` (we don't want the
   daemon to render text; we want the `Report` object), serialize the
   returned `Report` via `doctor.RenderJSON` (or just
   `json.NewEncoder(w).Encode(rep)` — same thing).
2. **Route registration.** Add
   `mux.HandleFunc("GET /v1/doctor", s.doctorHandler)` in
   `internal/agentd/server.go` adjacent to the existing `/v1/version`,
   `/v1/facts`, `/v1/metrics` lines.
3. **Client method.** Add `Client.Doctor(ctx, opts) (*doctor.Report, error)`
   to `internal/fleet/transport/agentd.go`. Maps to GET; encodes the
   query params; decodes the response body into `doctor.Report`. The
   transport package may need to import `internal/doctor` for the
   `Report` struct — if that introduces a cycle, define a thin
   `transport.DoctorReport` mirror and convert at the call site.
4. **Fleet fan-out helper.** New `internal/fleet/doctor.go`:
   - `type PeerDoctor struct { Peer Peer; Report *doctor.Report; Accessible, Unavailable bool; Error string }`
   - `RunAll(ctx, peers []Peer, opts DoctorOptions) []PeerDoctor`
     parallel across peers, bounded by `opts.Parallel`, per-peer
     `opts.Timeout`. Treats HTTP 404 distinctly from transport errors
     and sets `Unavailable=true`. Skips non-agentd peers with
     `Accessible=false, Error="non-agentd transport"`.
5. **CLI command.** New `cmd/fleet_doctor.go`:
   - Define `fleetDoctorCommand()` and register it in `cmd/fleet.go`'s
     `Subcommands` list.
   - Parse flags (`--peer`, `--peers`, `--peer-filter`, `--section`,
     `--detail`, `--json`, `--strict`, `--timeout`, `--parallel`,
     `--no-color`).
   - `--peer` set → fetch one peer, render via `doctor.RenderText` (or
     `RenderJSON` if `--json`); exit code from the embedded `Report`.
   - Else → fan out via `RunAll`, render aggregated table, exit code
     via a new `doctorExitCode(rows, strict)` helper.
6. **Tests.**
   - `internal/agentd/handlers_test.go`: doctor endpoint returns a
     valid `Report`; respects `section=` filter; rejects without bearer.
   - `internal/fleet/transport/agentd_test.go`: `Client.Doctor` against
     an `httptest.Server` returning a canned report; 404 path.
   - `internal/fleet/doctor_test.go`: `RunAll` aggregation across two
     fake peers (one returning 404 to verify the unavailable branch,
     one returning a report with warnings).
   - `cmd/fleet_doctor_test.go`: aggregated table snapshot test; JSONL
     output shape; exit codes for the three regimes (clean / warnings /
     errors / unreachable).

---

## Open questions

1. **Exit code for unreachable peers — 1 or 2?** This spec proposes
   exit 1 (different from `fleet status`'s exit 2) because "unreachable"
   is a doctor *finding* in itself. But mixed exit-code semantics across
   fleet subcommands is friction for scripts. Lean: revisit after one
   week of dogfooding — operator instinct on `$?` after `fleet doctor`
   will tell us.
2. **Should the daemon expose more than the kernel doctor catalogue?**
   The brainstorm asks: "daemon-internal health (queue depth, goroutine
   count, recent errors) on top of the kernel doctor checks?" The
   pragmatic answer is yes — `services/agentd` already lives in the
   catalogue, and extending it with `queue_depth_elevated`,
   `recent_run_failures > N` checks belongs in `internal/doctor/`
   (specifically `checks_services.go`), not in this spec. That work is
   a follow-on to the kernel doctor, gated by usage data. v1 of fleet
   doctor exposes exactly what `mooncake doctor` already does.
3. **What's the right per-peer timeout default?** 5s feels generous;
   200ms-per-check × 16 checks = 3.2s worst case, plus HTTP round trip.
   But disk-space and service checks can be slow on a busy peer.
   Defer real value to dogfooding; configurable via `--timeout`.
4. **Section names — should the daemon validate them?** Today
   `mooncake doctor --section foo` silently runs no checks (the
   selector filters to a non-existent section). The daemon could
   reject unknown section names with HTTP 400 to surface typos to the
   controller. Lean: yes — make it strict.
5. **Should `fleet doctor` include the controller-local doctor as one
   of the rows?** `fleet status` includes the local peer when agentd
   runs on the unix socket. For doctor, the answer is murkier — the
   *controller*'s `mooncake doctor` would re-run all the local checks
   the operator already has access to. Lean: no for v1, unless the
   controller is itself paired into `peers.toml` (in which case it's
   already a peer and gets fanned to like any other).
6. **`--checks <name>` (single-check drill-down).** The brainstorm
   mentions this. The kernel doctor doesn't currently have a "run
   one named check" filter — only sections. Adding a name filter is
   a kernel-doctor change. Defer to a follow-up spec that lands
   `--check` (singular) across both `mooncake doctor` and
   `fleet doctor`.
7. **Caching.** Should `fleet doctor` write last-seen results to
   `~/.cache/mooncake/fleet-doctor.json` so a shell prompt can read
   them without re-probing? Mirrors the `--cache` / `--from-cache`
   discussion in spec-46. v1: no cache; revisit if requested.
8. **Color theme in the aggregated table.** The PASS/WARN/FAIL columns
   want color (green/yellow/red). Same palette as `fleet status`'s
   ACCESSIBLE column. No new abstraction needed.

---

## Success criteria

After this spec lands:

1. `mooncake fleet doctor` against a 3-peer fleet completes in well
   under 10s and prints a per-peer summary table.
2. A peer running agentd at the version this spec ships in returns its
   doctor report over `GET /v1/doctor` in under 1s on a healthy box.
3. A peer running an older agentd surfaces as "doctor unavailable"
   without failing the command; the rest of the fleet still reports.
4. `mooncake fleet doctor --peer <name>` renders byte-identical output
   to `mooncake doctor` run locally on `<name>` (modulo the one-line
   banner identifying the peer).
5. `mooncake fleet doctor --json` parses with `jq` and exposes the same
   `Report` shape the local `mooncake doctor --format json` exposes,
   one record per peer.
6. The exit-code shape lets a shell script discriminate clean / warn /
   fail / unreachable without parsing the table.
