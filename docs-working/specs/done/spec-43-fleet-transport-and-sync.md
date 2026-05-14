# Spec 43: Fleet Network Transport + File Sync + `fleet apply`

**Epic:** Personal Fleet — see [`../../epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md), sub-epic P1.
**Status:** ✅ Shipped 2026-05-14. PR1-5 → 850b65c, PR6 (parallel multiplexer + ^C banner) → d17953f, post-merge polish (empty stdout, peer-filter typo warning, Windows config path, SSE race regression tests) → bd4695a.
**Effort:** L (~1–2 weeks)
**Value:** Very High — the everyday path. Turns the existing single-host
agentd into a fleet runtime: a controller writes a plan once, syncs it to
N peers, applies in parallel, and watches interleaved logs scroll in one
terminal.

---

## Problem

Today `mooncake agentd` exposes a working run/SSE surface — but only over a
local unix socket. Three things missing for the personal-fleet UX:

1. **Network transport** — `agentd` doesn't listen on TCP, so the controller
   cannot reach it from another machine.
2. **Plan delivery** — `POST /v1/runs` requires `plan_path` to be absolute on
   the *daemon's* filesystem. A remote controller has no way to get its
   `~/dotfiles/config.yml` onto the peer.
3. **Multiplexed log UX** — there's no `mooncake fleet apply` command; SSE
   exists per-peer but nothing combines streams from N peers into a single
   interleaved view.

The epic settled the architectural question (file sync into `<state_dir>/synced/`
+ run from there). This spec implements all three pieces.

---

## Goals

- **G1** Add a TCP listener to `agentd` alongside the unix socket, gated by
  a bearer-token auth middleware.
- **G2** Add a `PUT /v1/files` endpoint that writes into a sandboxed
  `<state_dir>/synced/<scope>/` subtree, plus `HEAD /v1/files` for skip-if-
  byte-identical sync.
- **G3** Add `mooncake fleet apply <plan>` CLI: walks the plan-dir, syncs to
  each configured peer (HEAD-skip + PUT), submits a run, and streams
  multiplexed SSE events to stdout with a `[hostname]` prefix.
- **G4** Keep the existing unix-socket path unchanged. Token auth applies only
  to the TCP listener.

**Non-goals (deferred to other specs):**

- Discovery (mDNS, SSH-config import) — spec-45.
- SSH transport / bootstrap — spec-44 / spec-47.
- Per-host overlays, tag selectors — spec-48.
- Fleet status, log reattach — spec-46.
- Run cancellation — out of scope; needs context plumbing through the
  executor and every action handler.
- TLS — assume LAN / Tailscale / WireGuard. Reconsider later.
- Cross-host DAG, wave rollouts — fleet runs are flat parallel in v1.

---

## Reuse map

This spec is mostly wiring. The hard parts already exist.

**Reused as-is:**

| File | What it provides |
|---|---|
| `internal/agentd/store.go` | Run records, `events.jsonl`, atomic writes, `Reconcile()`. |
| `internal/agentd/worker.go` | Single-goroutine FIFO worker, base_dir chdir, publisher → sink → terminal. |
| `internal/agentd/sse_hub.go` | Per-run broadcaster, `Subscribe()` returning `(ch, lastSeq, unsub)`. |
| `internal/agentd/jsonl_sink.go` | JSONL writer with seq numbers + hub broadcast. |
| `internal/agentd/runs_handler.go:runEventsHandler` | `GET /v1/runs/{id}/events` — replay-then-tail SSE. Untouched. |
| `internal/agentd/runs_handler.go:submitRunHandler` | `POST /v1/runs` — untouched; the synced path is a regular absolute path under it. |
| `internal/agentd/middleware.go` | Request ID, access log, recover. `statusRecorder` forwards Flush so SSE survives wrapping. |

**Extended:**

| Where | Change |
|---|---|
| `internal/agentd/config.go` | Add `BindAddr string`, `TokenPath string`, `MaxSyncBytes int64`. |
| `internal/agentd/server.go` | Open a TCP `net.Listen` alongside the unix listener. Same `routes()` mux serves both. Apply auth middleware only on the TCP path. |
| `internal/agentd/middleware.go` | New `bearerAuthMiddleware` (constant-time compare). |
| `cmd/agentd.go` | Add `--bind`, `--token-file` flags. |

**New:**

| Component | Location |
|---|---|
| `PUT/HEAD /v1/files` handler + path sandbox | `internal/agentd/files_handler.go` |
| Token generation + persistence | `internal/agentd/token.go` |
| `mooncake fleet` CLI shell | `cmd/fleet.go` |
| Peers TOML loader/writer | `internal/fleet/peers.go` |
| Controller ID minting | `internal/fleet/controller.go` |
| HTTP+SSE client to a peer | `internal/fleet/transport/agentd.go` |
| Plan-dir walker + sync loop | `internal/fleet/sync.go` |
| Interleaved log multiplexer | `internal/fleet/multiplex.go` |

---

## Wire shape

### `PUT /v1/files`

```
PUT /v1/files?scope=<scope>&path=<relative>
Authorization: Bearer <token>
Content-Type: application/octet-stream
X-Sha256: <hex>      ← optional integrity check
<body: file contents>
```

- `scope` and `path` are required query params.
- `scope` must match `[A-Za-z0-9_-]{1,128}` (controller id segment + dir-hash
  segment, joined by `/`). The handler rejects any other charset.
- `path` is interpreted relative to `<state_dir>/synced/<scope>/`. The
  handler computes the final path with `filepath.Clean`, then verifies it
  has the synced root as a prefix. Reject on:
  - `..` segments or absolute paths
  - any symlink in the final resolved path
  - path length > 1024 bytes
- Body is streamed to a temp file in the same directory, then atomically
  renamed into place. Mode 0600. Parent dirs created as needed (mode 0700).
- If `X-Sha256` is present, the handler hashes while writing and rejects
  with 422 if the hash mismatches.
- Body size enforced by `http.MaxBytesReader(cfg.MaxSyncBytes)`. Default
  100 MiB per file.

Responses:

- `204 No Content` on success.
- `400` on validation failures (path escape, charset, missing params).
- `401` if bearer missing/invalid.
- `413` if body exceeds limit.
- `422` on sha256 mismatch.

### `HEAD /v1/files`

```
HEAD /v1/files?scope=<scope>&path=<relative>&sha256=<hex>
Authorization: Bearer <token>
```

- `200 OK` if a file exists at the resolved path AND its sha256 matches.
- `404 Not Found` otherwise.

This is the cache hit endpoint. Controller calls it before each PUT; on 200
it skips the upload.

### Existing endpoints — unchanged

| Endpoint | Used for |
|---|---|
| `POST /v1/runs` | Controller submits with `plan_path = <state_dir>/synced/<scope>/<rel>`. The existing absolute-path check passes naturally. |
| `GET /v1/runs/{id}/events` | Controller subscribes for SSE log stream. |
| `GET /v1/facts` | Cheap peer probe (used by fleet status; not in this spec). |
| `GET /v1/version` | Liveness check. |

### Auth model

- Daemon generates a token at first start: 32 random bytes, base64url
  encoded, written to `<token_path>` (default `~/.config/mooncake/agentd.token`)
  with mode 0600. Idempotent: if the file exists and is non-empty, the daemon
  reads it and reuses it.
- TCP listener is wrapped in `bearerAuthMiddleware`. Unix socket listener is
  NOT wrapped — filesystem permissions on the socket (`0o600`) gate it.
- Constant-time compare (`crypto/subtle.ConstantTimeCompare`).
- No CORS, no preflight; this is not a browser endpoint.

---

## Controller side

### Controller identity

- A UUIDv4 is minted at first use of any `mooncake fleet` command and
  persisted at `~/.config/mooncake/controller_id` (mode 0600).
- Reused for the lifetime of the file. Deleting the file forces a new id
  on next run, which invalidates remote sync caches (cheap — they
  re-upload).

### Peers config

`~/.config/mooncake/peers.toml`:

```toml
# Peers controllable from this machine.

[[peers]]
name      = "macbook"             # display name in logs / fleet output
addr      = "macbook.lan:7878"    # host:port reachable via agentd HTTP
transport = "agentd"               # "agentd" | "ssh" (future)
token     = "abc123…"             # bearer for this peer (from peer's agentd.token)
tags      = ["workstation", "darwin"]
```

Loader: `internal/fleet/peers.go`. Validation:

- `name` must match `[a-zA-Z0-9._-]{1,64}` (used in `[host]` log prefix and
  as a path segment).
- `addr` must parse as `host:port`.
- `token` is mandatory for `transport = "agentd"`.

### Scope key derivation

```
controller_id = <UUID from ~/.config/mooncake/controller_id>
plan_dir      = <absolute path of the directory containing the top-level YAML>
dir_hash      = sha256(plan_dir)[:16]  # 16 hex chars
scope         = "<controller_id>/<dir_hash>"
```

Stable across runs from the same machine against the same dotfiles dir.

### Sync algorithm

```go
func Sync(planDir string, peer Peer, scope string) error {
    files, total := walkPlanDir(planDir, cfg.MaxSyncBytes)
    if total > cfg.MaxSyncBytes {
        return errors.New("plan-dir exceeds --max-sync-size")
    }
    for _, f := range files {
        sha := sha256File(f.AbsPath)
        if peer.HeadFile(scope, f.RelPath, sha) == 200 {
            continue // already on peer, byte-identical
        }
        peer.PutFile(scope, f.RelPath, f.AbsPath, sha)
    }
    return nil
}
```

`walkPlanDir`:
- Walk the plan-dir tree using `filepath.WalkDir`.
- Skip dotfiles by default *only at the top level*: `.git`, `.DS_Store`.
  Everything else is included.
- Refuse to follow symlinks that point outside the plan-dir.
- Accumulate cumulative size; abort if `> MaxSyncBytes`.

### Multiplexed log streaming

Each peer's SSE connection runs in its own goroutine; events fan in to a
single channel; one writer drains it to stdout. Format per event:

```
[<hostname>] <human-readable event summary>
```

Hostname column is left-padded to `max(len(peer.Name) for peer in peers)`.
Color per peer: stable hash of name → ANSI palette (8 colors). Disabled if
`NO_COLOR=1` or stdout is not a TTY.

Disconnect / completion:

- When a peer's SSE connection closes with the run in terminal state:
  emit one final line `[<hostname>] run complete: <changed> changed, <failed> failed`
  using the persisted run record.
- When a peer disconnects mid-stream: emit `[<hostname>] *** disconnected ***`
  and keep draining other peers.

`^C` handling:

- First `^C`: cancel the local SSE subscriptions (close streams). Print:
  ```
  ⚠ ^C closes the log stream only — remote runs continue.
    See `mooncake fleet logs <host>` to reattach.
  ```
- Second `^C`: hard exit. The fleet process dies; peers keep going.

This is documented honestly: there is no remote cancellation today.

### Exit code

`max(per-peer exit code)`. Per peer:
- `0` if run completed with `success` status.
- `1` if any peer's run ended in `failed` or `interrupted`.
- `2` if any peer was unreachable for sync or submit.

---

## CLI surface

```
$ mooncake fleet apply <plan.yml> [flags]
  --peers <name,name,...>   Limit to named peers (default: all in peers.toml)
  --fresh                   Wipe the remote scope before sync
  --max-sync-size <bytes>   Override daemon-default 100 MiB cap (controller side)
  --parallel <N>            Max peers in flight at once (default: all)
  --no-color                Disable hostname colors
```

```
$ mooncake agentd
  ...existing flags...
  --bind <addr>            TCP bind address (e.g. 0.0.0.0:7878); empty = unix only
  --token-file <path>      Token file path (default ~/.config/mooncake/agentd.token)
  --max-sync-bytes <N>     Per-file PUT size cap (default 100 MiB)
```

---

## Tasks

### Task 1 — agentd TCP listener + bearer auth

1. Add `BindAddr`, `TokenPath`, `MaxSyncBytes` fields to `agentd.Config`.
2. Add `--bind`, `--token-file`, `--max-sync-bytes` flags in `cmd/agentd.go`.
3. New `internal/agentd/token.go`: `LoadOrCreateToken(path string) (string, error)`.
   First call writes 32 random bytes → base64url → file 0600. Subsequent
   calls read and return the same token.
4. New `bearerAuthMiddleware` in `middleware.go` using
   `crypto/subtle.ConstantTimeCompare`. Returns 401 on mismatch.
5. `server.go:Serve` opens a second listener when `cfg.BindAddr != ""`.
   Same `routes()` mux. Auth-wrap the TCP-only path; unix path stays
   unauthenticated. Shut both down on context cancel.

### Task 2 — File-sync endpoint

1. New `internal/agentd/files_handler.go`:
   - `(s *Server) putFileHandler(w, r)`.
   - `(s *Server) headFileHandler(w, r)`.
2. Path validation in a helper `resolveSyncPath(stateDir, scope, rel) (string, error)`:
   - Validate `scope` against `^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)?$`.
   - Reject if `rel` contains `..` after `Clean`.
   - Reject if `filepath.Join(...)` doesn't have the synced root as a strict prefix.
   - Reject if any path component is a symlink (use `os.Lstat` walking up).
3. Streaming write: `http.MaxBytesReader` for size cap, hash while writing,
   write to `<final>.tmp.<random>`, fsync, rename. Best-effort rename
   cleanup on error.
4. Wire routes in `server.go:routes()`:
   ```go
   mux.HandleFunc("PUT  /v1/files", s.putFileHandler)
   mux.HandleFunc("HEAD /v1/files", s.headFileHandler)
   ```

### Task 3 — Fleet CLI scaffold

1. New `cmd/fleet.go`: `fleetCommand()` returns a `*cli.Command` with
   subcommands. v1 has `apply` only; later specs add `status`, `logs`, etc.
2. Wire into `cmd/mooncake.go:Commands` next to `agentdCommand()`.
3. New `internal/fleet/peers.go`: TOML loader for `~/.config/mooncake/peers.toml`.
4. New `internal/fleet/controller.go`: `EnsureControllerID() (string, error)`
   minting the UUIDv4 once and persisting it.

### Task 4 — Peer transport client

1. New `internal/fleet/transport/agentd.go`. Methods on a `*Peer`:
   - `Head(scope, relPath, sha256) (bool, error)` → HEAD /v1/files.
   - `Put(scope, relPath, srcAbs, sha256) error` → PUT /v1/files.
   - `Submit(planPath string) (runID string, err error)` → POST /v1/runs.
   - `Stream(ctx, runID string, sink chan<- Event) error` → GET /v1/runs/{id}/events.
2. Each method sets `Authorization: Bearer <peer.Token>`. Body size bounded
   on both sides. Reasonable timeouts on the non-SSE calls (10s connect,
   30s overall); SSE has no read deadline by design.
3. SSE parser: `bufio.Scanner` over `text/event-stream`; assemble multi-line
   `data:` fields; emit one event per blank-line delimiter. Decode the JSON
   `data` payload into an existing `events.Event`-like struct.

### Task 5 — Sync loop

1. New `internal/fleet/sync.go`. `Walk(planDir, max int64) ([]FileEntry, int64, error)`
   enforcing the cumulative-size cap and skipping `.git`, `.DS_Store`.
2. `SyncTo(peer Peer, planDir, scope string) error` runs HEAD then PUT.
3. Build the synced absolute path on the peer side:
   `peer-side-state-dir/synced/<scope>/<rel-of-top-yaml>`. The controller
   must know the peer's state dir — fetched via `GET /v1/version` extended
   with a `state_dir` field (small EXTEND in this spec; add `SyncedRoot` to
   the version response).

### Task 6 — Multiplexer + apply orchestration

1. New `internal/fleet/multiplex.go`. `Run(ctx, peers []Peer, eventCh <-chan PeerEvent, out io.Writer)`:
   - Compute prefix padding from `max(len(p.Name))`.
   - Stable color map by name hash (or no color if `NO_COLOR` / non-TTY).
   - Forward each event as `[name] <line>\n`.
2. New `cmd/fleet.go:applyAction`:
   1. `EnsureControllerID()` → controller_id.
   2. Resolve plan-dir from arg; sha256 the absolute path; compute scope.
   3. Load `peers.toml`; filter by `--peers` if given.
   4. For each peer, in parallel up to `--parallel`:
      a. `SyncTo(peer, planDir, scope)` (PUT all needed files).
      b. `peer.Submit(syncedTopPath)` → run_id.
      c. `peer.Stream(ctx, run_id, eventCh)`.
   5. Wait for all per-peer streams to terminate.
   6. Print summary, return aggregate exit code.
3. Handle SIGINT: cancel the local context, print the "remote runs continue"
   banner once, wait briefly for streams to flush, exit.

### Task 7 — Tests

- `internal/agentd/files_handler_test.go`:
  - Reject `..` escapes, absolute paths, symlinked parents.
  - Round-trip a 1 MiB file; verify byte-identical on disk.
  - HEAD returns 200 only when sha256 matches exactly.
  - Body > `MaxSyncBytes` → 413.
- `internal/agentd/auth_test.go`:
  - TCP request without token → 401.
  - With wrong token → 401.
  - With right token → routes through.
  - Unix request without token → 200 (unaffected).
- `internal/fleet/sync_test.go`:
  - Walker respects cumulative cap.
  - HEAD-skip on a second sync of the same dir.
- `internal/fleet/multiplex_test.go`:
  - N peers, deterministic input → expected interleaved output (use a fake
    clock + fixed peer ordering to make stable assertions).
- End-to-end integration test under `cmd/`: spin up two agentd's on
  random ports, `fleet apply` a trivial plan, assert exit code 0 and that
  both peers emitted run-complete events.

---

## File layout after this spec

```
~/.config/mooncake/
  agentd.token              ← daemon's bearer (mode 0600)
  controller_id             ← controller's UUIDv4
  peers.toml                ← controller's peer list

~/.local/state/mooncake/agentd/    ← <state_dir>, set by XDG
  runs/<run_id>/                   ← existing
    record.json
    events.jsonl
  synced/<controller_id>/<dir_hash>/   ← new
    config.yml
    presets/...
    vars/...
```

---

## Open questions

1. **Peer state_dir discovery.** `GET /v1/version` is the simplest place to
   surface `synced_root`. Alternative: a fixed convention (`~/.local/state/mooncake/agentd/synced` always). Pick one in implementation; the
   version-field approach is more honest about system mode.
2. **GC of old scopes.** No GC in v1. A user who renames their dotfiles dir
   leaves an orphan scope on every peer. Could ship a `mooncake fleet gc`
   command later, or auto-GC scopes older than 30 days.
3. **Concurrent applies to the same scope.** Two `fleet apply` invocations on
   the same controller, against the same dir, racing PUTs on the same peer.
   Simplest answer: per-(scope, path) lock on the daemon side, blocking the
   second PUT briefly. Or accept clobber and document it.
4. **PUT atomicity across files.** Right now PUTs are independent. If sync
   fails halfway, the peer has a partial tree. Mitigation: name the scope
   directory with a `.tmp.` prefix during sync, rename on success. Adds
   complexity; defer unless we hit a real failure mode.
5. **Disabling unix socket.** When `--bind` is set without `--socket=...`,
   should the unix socket still bind by default? Lean yes — local
   `mooncake apply` keeps working. Make it opt-out (`--no-unix`).
