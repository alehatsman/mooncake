# Epic: Personal Fleet — Mooncake for the Machines You Already Own

> Status: brainstorming / future epic. Sibling to
> [`epic-cluster-management.md`](epic-cluster-management.md): same primitives,
> different audience. Iterate here before moving to formal specs under
> `docs-working/specs/`.

---

## The thesis

> **Mooncake should make controlling 1–10 personal machines from a single
> terminal feel as natural as controlling one.**

You — the user — have a MacBook, an Arch laptop (X1), and two desktop PCs.
You want to write one mooncake config, apply it to all four, watch the
combined log scroll past in your terminal, and know at a glance which boxes
are healthy and which have drifted. No hub, no auth server, no provisioning
pipeline. Just `mooncake fleet apply` from any machine to any other.

This epic is **the developer-experience layer** on top of agentd. The
[enterprise cluster-management epic](epic-cluster-management.md) covers the
*platform-team* fleet story (hub, RBAC, drift heatmaps, AI remediation). This
one covers the *solo-developer* fleet story (peer-to-peer, no central server,
"my own machines on my own network"). Same kernel, different scope, different
ergonomics.

It is also Stream 4 (Developer Experience) made concrete — Stream 4 has had
zero active specs; this epic gives it a flagship deliverable.

---

## Driving use case

```
                     ┌──────────────┐
                     │   laptop     │ ← you sit here
                     │   (arch x1)  │   `mooncake fleet apply config.yml`
                     └──────┬───────┘
                            │ LAN / SSH / mDNS
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
  ┌──────────┐        ┌──────────┐        ┌──────────┐
  │ desktop1 │        │ desktop2 │        │ macbook  │
  │ (arch)   │        │ (arch)   │        │ (darwin) │
  └──────────┘        └──────────┘        └──────────┘
```

What success looks like in a single terminal session:

```text
$ mooncake fleet apply ~/dotfiles/config.yml
discovering peers … 4 found (laptop, desktop1, desktop2, macbook)
[laptop]   apply file.copy ~/.zshrc            → unchanged
[desktop1] apply file.copy ~/.zshrc            → changed
[macbook]  apply file.copy ~/.zshrc            → changed
[desktop2] apply pkg.install neovim            → ok
[laptop]   apply pkg.install neovim            → unchanged
[macbook]  apply pkg.install neovim            → changed
[desktop1] apply pkg.install neovim            → changed
[desktop2] run complete: 7 changed, 0 failed   ✓
[macbook]  run complete: 9 changed, 0 failed   ✓
[laptop]   run complete: 2 changed, 0 failed   ✓
[desktop1] run complete: 11 changed, 0 failed  ✓
fleet apply: 4/4 ok, 29 changed total, 8.4s
```

That is the "see logs from remote in terminal" moment. Interleaved, prefixed,
greppable, exit code = worst case.

---

## How this differs from the enterprise epic

| Dimension | Enterprise epic (cluster-management) | This epic (personal-fleet) |
|---|---|---|
| Audience | Platform teams, 50–10k nodes | Solo devs, 1–10 nodes |
| Control plane | Central hub, persisted inventory | Peer-to-peer, no hub |
| Identity | mTLS / SSO / per-user RBAC | Trust-on-first-use bearer token |
| Discovery | Nodes register with hub | Local LAN (mDNS) + SSH config + static peers |
| Transport | Long-lived agentd over network | Same agentd, plus first-class **SSH fallback** for unprovisioned boxes |
| Approvals | Approval gates, change windows | None — you trust yourself |
| Drift UX | Heatmap dashboard, paging alerts | `mooncake fleet status` table |
| Audit | Signed exportable log | Local JSONL on each peer, queryable |
| Remediation | AI proposes, human approves, hub dispatches | `apply` is the remediation; rerun whenever |
| Persistence | SQLite → Postgres | Filesystem per peer, no shared store |

They are not in competition. The enterprise hub, when it ships, treats a
personal-fleet peer as a degenerate "1-node region" and vice-versa. The
on-the-wire protocol is the same.

---

## Reuse map — what already exists

A careful read of `internal/agentd/` shows the kernel is in much better shape
than the headline "agentd is unix-socket only" suggests. ~80% of the
network-transport story is already written; the missing pieces are the
listener flavor, an auth middleware, the controller-side code, and one
architectural decision (next section).

### REUSE as-is (production quality, no changes needed)

| File | What it gives the personal-fleet epic |
|---|---|
| `internal/agentd/store.go` | ULID-ordered run records, atomic temp+rename writes, `Reconcile()` marks orphaned runs `interrupted` on daemon restart. The per-node audit trail is done. |
| `internal/agentd/worker.go` | Single-goroutine FIFO worker; chdirs to `base_dir`; drains publisher → JSONL sink → terminal record in the documented critical order. |
| `internal/agentd/sse_hub.go` | Per-run in-memory broadcaster. `Subscribe()` returns `(ch, lastSeq, unsub)` — exactly what the controller's per-peer reader will consume. Drops on slow subscriber rather than blocking the broadcaster. |
| `internal/agentd/jsonl_sink.go` | Per-run subscriber that writes the same bytes to both `events.jsonl` and the hub, with monotonic `seq` numbers. Solves replay→tail duplicate-event boundaries already. |
| `internal/agentd/runs_handler.go:runEventsHandler` | **`GET /v1/runs/{id}/events`** — full SSE replay-then-tail handler with seq-based dedup. This is the wire format the controller subscribes to. No changes needed. |
| `internal/agentd/middleware.go` | Request ID + access log + recover. `statusRecorder.Flush()` already delegates so SSE survives the access-log wrapper. |
| `internal/mcp/tools.go` + `RegisterAllTools` | Already wired into agentd's `POST /v1/mcp` via `DispatchBytes`. Reusable for `mooncake fleet facts <host>`, `mooncake fleet snapshot <host>`, etc. |

### EXTEND (small additive changes, no behavior change)

| Where | Change | Effort |
|---|---|---|
| `internal/agentd/config.go` | Add `BindAddr string` and `Token` / `TokenPath`. Socket path stays optional. | XS |
| `internal/agentd/server.go:Serve` | Open a second `net.Listen("tcp", cfg.BindAddr)` alongside the unix listener; same `routes()` mux serves both. | S |
| `internal/agentd/middleware.go` | New `bearerAuthMiddleware`, attached only to the TCP listener — unix stays unauth'd (filesystem perms gate it). | XS |
| Token file management | Generate `agentd.token` (32 random bytes, base64) at first start under `~/.config/mooncake/`, mode 0600. | XS |
| New route `GET /v1/peers/self` *(optional)* | Hostname + tags + version for discovery probes. | XS |

All of P1's daemon-side work, modulo testing, is about a day of code.

### NEW (green-field — doesn't exist anywhere)

| Component | Location | Effort |
|---|---|---|
| `mooncake fleet` CLI subcommands | `cmd/fleet.go` | M |
| `peers.toml` loader/writer | `internal/fleet/peers.go` | S |
| mDNS discovery (advertise + query) | `internal/fleet/discovery/mdns.go` + agentd-side advertise | M |
| SSH-config importer | `internal/fleet/discovery/sshconfig.go` | S |
| HTTP+SSE client (`POST /v1/runs`, consume `/events`) | `internal/fleet/transport/agentd.go` | S |
| File-sync endpoint `PUT /v1/files` + path sandbox under `<state_dir>/synced/` | `internal/agentd/files_handler.go` | S |
| Plan-tree walker + sync client | `internal/fleet/sync.go` | S |
| SSH bootstrap (binary install + token pairing) | `internal/fleet/transport/ssh.go` | S |
| Multiplexer: N×SSE → interleaved `[host] line` stdout | `internal/fleet/multiplex.go` | M |

The multiplexer is the only piece that's genuinely tricky, and even it is
shielded from event-format concerns because `seq`-numbered JSON lines are
already the wire format coming off `runEventsHandler`.

---

## Architectural decision: plan transport (resolved)

Today the daemon requires the plan to **already exist on its own filesystem**.
From `internal/agentd/runs_handler.go`:

```go
if !filepath.IsAbs(req.PlanPath) {
    writeError(w, 400, "relative_plan_path", "plan_path must be absolute")
    return
}
info, err := os.Stat(planPath)   // ← stat on the daemon's FS, not the controller's
```

`vars_files` and `base_dir` have the same constraint. The MCP `run_plan` tool
in `internal/mcp/tools.go` has the same constraint. Spec-18 explicitly
deferred this: *"Preset bundling in the apply payload (presets must already
exist on target)."*

That assumption is fine for **a user driving their own daemon on their own
box**. It is fatal for the personal-fleet promise of "write a plan on my
laptop, watch it apply on my macbook," because the macbook does not have
`~/dotfiles/config.yml` until something puts it there.

### Chosen approach: file sync into agentd's state dir, then run as usual

**Decision**: the controller copies the plan tree (top-level YAML +
referenced presets + vars files) into a known location on the peer —
**`<state_dir>/synced/<scope>/`**, as a sibling of the existing
`<state_dir>/runs/` tree — and then submits a regular `POST /v1/runs`
pointing at the synced absolute path. No tarball, no inline payload, no
extraction step. The existing run-submit handler is untouched; the peer's
filesystem ends up containing exactly what the controller had, in a visible,
debuggable location.

`<state_dir>` is whatever agentd is configured with (`config.go:Default`):

- User mode (default): `$XDG_STATE_HOME/mooncake/agentd/` → typically
  `~/.local/state/mooncake/agentd/`
- System mode: `/var/lib/mooncake/agentd/`

Synced plans therefore land at `~/.local/state/mooncake/agentd/synced/<scope>/`
in the common case, alongside `~/.local/state/mooncake/agentd/runs/`. One
state dir, two children: `runs/` and `synced/`. No new top-level dotfile.

**Sync set:** the entire plan-dir, recursively. The controller treats the
directory containing the top-level YAML as the unit of sync and mirrors it
verbatim. This includes `./presets/`, `./vars/`, `./templates/`, and any
auxiliary files referenced by `file.copy` actions. **System presets**
(`/usr/local/share/mooncake/presets/`, `~/.mooncake/presets/`) are not synced
— they ship with the peer's mooncake install. A `--max-sync-size` safety belt
(default 100 MB) rejects pathological plan-dirs before walking them.

**Wire shape (sketch):**

- `PUT /v1/files?scope=<scope>&path=<relative>` — body is file contents.
  Daemon writes to `<state_dir>/synced/<scope>/<relative>` after rejecting
  any path that escapes the synced root (`..`, absolute paths, symlinks).
- `HEAD /v1/files?scope=<scope>&path=<relative>&sha256=<hex>` returns 200 if
  the byte-exact file is already on disk, 404 otherwise. Lets the controller
  skip uploads on rerun — first step toward a content-addressed cache without
  committing to one now.
- Controller side: `mooncake fleet apply` walks the plan dir, HEAD-skips
  unchanged files, PUTs the rest, then `POST /v1/runs` with the synced
  absolute path as `plan_path`.

**Scope key:** `<controller_id>/<sha256(abs(plan-dir))[:16]>`. The
**controller_id** is a UUIDv4 minted on first use and persisted at
`~/.config/mooncake/controller_id`. Same controller + same plan-dir → same
scope → incremental sync. A `--fresh` flag wipes the remote scope before
sync if you want a clean slate.

**Why this beats the alternatives:**

| Option | Why not the answer |
|---|---|
| A. Inline `plan_yaml` body | Breaks the moment the plan `include:`s a preset. Trap for the trivial demo case. |
| B. Tarball bundle endpoint | Adds tar construction on the controller, extraction + GC on the daemon, and a bundle lifecycle to manage. All to deliver files we could PUT individually. Reconsider if individual PUTs prove slow. |
| C. SSH-only (sidestep) | Still useful for bootstrap (P2). But makes agentd-managed peers feel second-class — the everyday `fleet apply` path shouldn't depend on having SSH set up. |
| **D. File sync + run** (chosen) | Daemon change is one new endpoint with path sandboxing. Controller change is "walk + PUT." Files land where you can `ls` them. Incremental sync falls out naturally. No new lifecycle concepts. |

**Why this is safe:** the synced tree is sandboxed under
`<state_dir>/synced/<scope>/` and the daemon refuses any `plan_path` outside
that subtree (or any pre-existing path the user owns — same allowlist as
today). `rm -rf ~/.local/state/mooncake/agentd/synced/` fully resets sync
state without touching run history. The location is XDG-correct and
introspectable; it sits next to `runs/` so "what is this daemon storing"
has exactly one answer.

**Followup options still open:**

- Move sync to **content-addressed** later (`PUT /v1/blobs/<sha256>` + a
  manifest) once we know the access pattern. Drop-in, no UX change.
- Cap per-peer disk usage and GC old scopes on a timer.
- File-watch on the controller so `fleet apply --watch` re-syncs+reruns on
  every save.

---

## Proposed sub-epics

All six have draft specs under [`docs-working/specs/personal-fleet/`](../specs/personal-fleet/).

- **P1** → [spec-43: Fleet Network Transport + File Sync + `fleet apply`](../specs/personal-fleet/spec-43-fleet-transport-and-sync.md)
- **P2** → [spec-44: SSH Bootstrap Transport](../specs/personal-fleet/spec-44-ssh-bootstrap-transport.md)
- **P3** → [spec-45: Fleet Discovery — mDNS + SSH Config + Static Peers](../specs/personal-fleet/spec-45-fleet-discovery.md)
- **P4** → [spec-46: `fleet status`, `fleet logs`, `fleet facts`](../specs/personal-fleet/spec-46-fleet-status-and-logs.md)
- **P5** → [spec-47: `fleet bootstrap` UX + Pairing](../specs/personal-fleet/spec-47-fleet-bootstrap-ux.md)
- **P6** → [spec-48: Per-Host Overlays + Tag Selectors](../specs/personal-fleet/spec-48-per-host-overlays-and-tags.md)

The sketches below remain for context; the specs are authoritative on
scope and design details.

### P1: agentd network transport + file sync + `mooncake fleet apply`

**Plumbing plus file sync.** The existing `/v1/runs` endpoint, SSE hub, and
JSONL sink do almost all the run-execution work; this sub-epic adds the
listener flavor, auth, controller-side client, multiplexer, and the new
file-sync endpoint described in the architectural-decision section.

Scope:

- **TCP listener on agentd** alongside the unix socket. New flags:
  `--bind 0.0.0.0:7878`, `--token-file ~/.config/mooncake/agentd.token`.
  Same `routes()` mux serves both listeners.
- **Bearer-token auth middleware** on the TCP listener (single shared token
  per peer; manual rotation in v1).
- **File-sync endpoint** `PUT /v1/files?path=<rel>` writing to
  `<state_dir>/synced/<scope>/<rel>` (typically
  `~/.local/state/mooncake/agentd/synced/<scope>/<rel>` in user mode), with
  strict path sandboxing (reject `..`, absolute paths, symlinks).
- **Optional `HEAD /v1/files`** with sha256 query for skip-if-already-synced.
- **`mooncake fleet apply <plan>`** on the controller:
  1. Resolve the plan tree on the controller (includes, presets, vars).
  2. Compute a stable scope key for the (controller, plan-dir) pair.
  3. For each peer, walk the plan tree and PUT each file (skip on
     matching `HEAD`).
  4. `POST /v1/runs` with the synced absolute path as `plan_path`.
  5. `GET /v1/runs/{id}/events` (SSE), multiplex into interleaved
     `[host] line` stdout.
  6. Exit code = max over all peers' exit codes.

The existing `runs_handler` is unchanged. The sandboxed sync root is the only
new daemon concept.

### P2: SSH transport — scoped to bootstrap

With file sync chosen as the everyday transport (see architectural-decision
section), SSH is no longer the daily-driver path. It collapses to one job:
**bootstrap a box that has no mooncake binary or agentd running yet.**

- `mooncake fleet bootstrap user@new-box` (the only entry point):
  - SCPs the local mooncake binary to `/tmp/` on the target.
  - SSH-runs the installer (places binary, writes launchd/systemd unit,
    starts agentd, prints token).
  - Reads the printed token back and writes it into the controller's
    `peers.toml`.
- One-off `mooncake fleet apply --via-ssh user@host config.yml` may still
  exist as an escape hatch for "agentd is broken, I need to fix it now," but
  it is not the recommended path and not part of the v1 demo.

This sub-epic is therefore much smaller than originally drafted — mostly a
shell-out wrapper around `scp` and `ssh`. The fancy file-streaming SSH client
work disappears because agentd-over-HTTP is what carries plans day-to-day.

### P3: Discovery

Three discovery sources, merged into a unified candidate list:

1. **`~/.config/mooncake/peers.toml`** — explicit, always wins. Generated by
   `fleet init` / `fleet bootstrap` and editable by hand.
2. **mDNS / DNS-SD** — agentd advertises `_mooncake._tcp.local` with its
   bearer-token fingerprint. Controller queries on LAN.
3. **SSH config import** — `mooncake fleet init` scans `~/.ssh/config` and
   offers to add hosts as `transport = "ssh"` peers (no agentd assumed).

```text
$ mooncake fleet init
scanning local network (mDNS) … 3 candidates: desktop1, desktop2, macbook
scanning ~/.ssh/config        … 2 candidates: vps-1, desktop2 (already found)
add all? [Y/n] Y
wrote ~/.config/mooncake/peers.toml (4 peers)
```

### P4: `mooncake fleet status` + live log reattach

The "at-a-glance health" command. One screen, one truth.

```text
$ mooncake fleet status
HOST       TRANSPORT  LAST-SEEN   OS        AGENTD   QUEUE  DRIFT
laptop     local      now         arch      0.9.0    0      —
desktop1   agentd     3s ago      arch      0.9.0    0      clean
desktop2   agentd     5s ago      arch      0.9.0    1      3 files
macbook    agentd     12s ago     darwin    0.9.0    0      clean
vps-1      ssh        —           —         —        —      —
```

- `mooncake fleet logs <host>` — attach to an in-flight run on that peer (SSE
  reconnect by run id; falls back to last completed run if none active).
- `mooncake fleet logs --all` — attach to all peers' current runs at once.
- `mooncake fleet facts <host>` — fetch facts for one peer.

### P5: `mooncake fleet bootstrap`

Zero-to-managed in one command. This is the surface for P2's SSH transport.

```text
$ mooncake fleet bootstrap user@new-box
detecting OS … darwin arm64
installing mooncake 0.9.0 … done
installing launchd plist  … done
starting agentd            … running on :7878
pairing                    … token written to ~/.config/mooncake/peers.toml
verifying                  … new-box responds, 1 fact collected (hostname)
✓ new-box is now part of your fleet
```

After this completes, the new box is reachable via the everyday agentd+file-sync
transport from P1. SSH is not used again until the next bootstrap.

### P6: Per-host overlays and tag selectors

The minimum-viable diversity story for "same plan, different boxes."

- `vars/by-host/<hostname>.yml` overlays on top of `vars/common.yml`.
- `peers.toml` carries free-form tags: `tags = ["workstation", "linux"]`.
- `mooncake fleet apply --tag os=darwin config.yml` filters peers.
- Hostname-based skip: `when: host == "macbook"` in plan steps.

Deliberately tiny. No `group_vars` / `host_vars` ansible-isms; just one
overlay file per hostname plus tag filtering.

---

## What stays out of scope

To keep this epic the *solo-dev* epic and prevent it from collapsing into
the enterprise one:

- **Central hub** — no shared server. Each peer is independent. (Enterprise
  epic handles this.)
- **RBAC / approval gates** — you trust your own machines.
- **SSO / OIDC** — single shared bearer token per peer is enough.
- **Drift heatmap UI / dashboard** — `fleet status` table is the UI.
- **AI-assisted remediation** — covered in safe-agent-runtime stream; not
  exclusive to fleet.
- **Cross-host DAG / wave rollouts** — for a 4-box fleet, parallel apply is
  fine. Add later if pain emerges.
- **Tailscale-specific integration** — works *over* Tailscale by virtue of
  being just HTTP, but no Tailscale-native identity in v1. Reconsider if
  users ask.
- **Push-mode "agentd phones home"** — peer is a server, controller is a
  client. Symmetric: any box can be controller, any box can be peer.

---

## Why peer-to-peer (not hub)

The hub is the right shape for a platform team running 500 nodes. It is the
wrong shape for a person running 4. A hub for personal use:

- Needs a "primary" machine that must stay up to coordinate.
- Adds a deployment artifact (hub binary + DB) the user has to babysit.
- Forces an authentication story (who is the hub talking to?) that is
  awkward when "you" are the only user.
- Centralizes state in a place that, if lost, breaks the fleet.

Peer-to-peer means: every box runs agentd. Any box can be the controller
for any apply. Nothing persists centrally that can't be recovered by asking
the peers. The controller is just whoever has a terminal open.

When the enterprise hub ships, a personal-fleet user can opt in by pointing
their agentds at the hub URL; nothing has to change in the protocol.

---

## Live-log streaming — controller-side multiplexer

The single most important DX moment. What makes this *feel* like one machine.

**The good news**: the per-peer wire format is already done. Each peer's
`GET /v1/runs/{id}/events` is a standard SSE stream of seq-numbered JSON
events, produced by `runEventsHandler` and the JSONL sink. Replay-then-tail,
dedup at the boundary, drop-on-slow-subscriber semantics — all decided and
shipped. The controller does not get to (or need to) renegotiate any of it.

**What's actually new** is the controller-side multiplexer. Per-peer goroutines
each `GET /v1/runs/{id}/events`, decode SSE frames, and forward decoded events
into a single output channel keyed by hostname. A single writer drains that
channel to stdout.

**Default (interleaved) output:**

- One goroutine per peer reads SSE and forwards to a central channel.
- Single writer to stdout, line-buffered, format: `[hostname] message`.
- Hostname column padded to the longest peer name for alignment.
- Color per hostname (stable hash → ANSI palette; disabled with `NO_COLOR`).
- On peer disconnect: emit `[host] *** disconnected ***` and continue draining
  other peers.
- **On controller `^C`**: close the SSE streams. Peer-side runs continue to
  completion — agentd v1 has no cancellation (`executor.Start` doesn't take
  a context, `worker.go:79` documents this). Output stops appearing in the
  terminal; the work doesn't roll back. Add a banner explaining this on the
  first `^C`. Real run cancellation is a separate spec touching the executor
  + every action handler; out of scope for personal-fleet v1.

**Future (`--tui`):** opt-in split-pane view. Not in v1 — interleaved covers
99% of real use.

**Piping & scripting:** because interleaved is plain stdout, this works:

```bash
mooncake fleet apply config.yml 2>&1 | grep -F '[macbook]' | grep ERROR
```

That property is lost the moment a TUI gets involved — argument for shipping
interleaved first.

---

## Trust model

For personal fleet, "trust on first use" with shared bearer per peer.

- Each agentd generates a long random token at first start; written to
  `~/.config/mooncake/agentd.token` (mode 0600).
- `mooncake fleet bootstrap` and `mooncake fleet pair` fetch the token over
  the secure channel (SSH for bootstrap, manual paste for pair) and write it
  into the controller's `peers.toml`.
- TLS optional in v1 — most personal fleets are on LAN or over Tailscale /
  WireGuard already. Add HTTPS-with-self-signed in P7 if anyone complains.
- Token rotation: edit `peers.toml`, restart agentd. Boring on purpose.

This is intentionally not enterprise PKI. It is the "you own all four
machines" trust model.

---

## Sequencing instinct

| Sub-epic | Dependency | Notes |
|---|---|---|
| P1 agentd network transport + file sync + `fleet apply` | agentd ✅, MCP ✅ | The everyday path; ~1 day daemon side, multiplexer is the bulk |
| P2 SSH (bootstrap-only) | nothing | Independent; small now that everyday path is file-sync over HTTP |
| P3 Discovery | static `peers.toml` works without P1; mDNS needs P1 | Three sources merged |
| P4 `fleet status` + logs reattach | P1 | Reuses SSE infra |
| P5 `fleet bootstrap` | P2 + P1 | SSH installs; then peer joins the agentd path |
| P6 Per-host overlays | P1 | Plan-compile-time concern |

Recommended ship order: **P1 → P2 → P3 → P5 → P4 → P6**.

Rationale flipped now that the plan-transport question is resolved: P1 is the
everyday path and it's small (existing run-handler reused, file-sync endpoint
is one new handler with path sandboxing, controller side is walk-and-PUT plus
multiplexer). Shipping P1 first proves the day-to-day UX directly. P2
becomes a smaller follow-up scoped to bootstrap only. P3/P5 make adding new
boxes painless. P4 polishes the inspection story. P6 is the cherry on top.

If P1 hits unforeseen complexity, P2 remains a viable parachute — SSH-only
mode still works without the file-sync endpoint, just with worse UX (no
incremental sync, no live SSE multiplex).

---

## Open questions

1. **mDNS daemon footprint** — embedding a Go mDNS library (e.g.
   `hashicorp/mdns`, `grandcat/zeroconf`) vs. shelling out to `avahi-publish`
   / `dns-sd`. Lean to embedding for portability; revisit if binary size
   becomes a complaint.
2. **What hostname does a peer advertise itself as?** OS hostname is messy
   (`MacBook-Air.local` etc). Idea: at `fleet init`, prompt for a short
   nickname per peer, store in `peers.toml` and as an agentd config field.
3. **Should `peers.toml` be the single source of truth, or should agentd
   maintain its own list of "who I trust to send me plans"?** Probably both:
   controller-side peer list = address book; agentd-side allowlist =
   accept-control-from list.
4. **Concurrency limit on `fleet apply`** — N peers in parallel by default?
   `--parallel 2` to throttle? For 4 boxes, full parallel is fine; for 20
   it's not.
5. **What happens when one peer fails mid-apply?** Halt-all (Ansible style),
   continue-others (kubectl style), or configurable? Lean to
   continue-others-by-default for personal fleet: you'd rather have 3/4
   succeed than 0/4.
6. **`mooncake fleet ssh <host>`** — proxy an interactive shell through
   agentd? Tempting but probably out of scope; you have ssh already.
7. **Drift on `fleet status`** — runs `check` against the last applied plan?
   Or the plan currently checked into `~/dotfiles`? The latter is more
   useful but requires the controller to know which plan to check against
   for each peer. Likely needs a per-peer `default_plan = "..."` in
   `peers.toml`.

---

## Success criteria for v1 of this epic

A user with the driving setup above (4 boxes, mixed Arch/macOS) can:

1. Run `mooncake fleet init` and end up with 4 working peers.
2. Run `mooncake fleet apply ~/dotfiles/config.yml` and watch interleaved
   logs scroll past, no failure, exit 0.
3. Run `mooncake fleet status` and see one healthy line per box.
4. Add a fifth machine with `mooncake fleet bootstrap user@newbox` in under
   60 seconds.
5. Pipe the apply output to `grep` and have it work because it's plain
   stdout.

If those five things work end-to-end on a Friday evening, this epic is done
in v1. Everything else is polish.

---

*Edit me freely. Cross out what's wrong, expand what's underdeveloped, fork
sub-epics into specs under `docs-working/specs/personal-fleet/` when they're
ready to move from "idea" to "plan."*
