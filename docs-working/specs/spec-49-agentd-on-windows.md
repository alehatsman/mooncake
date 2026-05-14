# Spec 49 — `agentd` on Windows

**Status:** Draft
**Effort:** S (~1–2 days)
**Value:** High — unlocks `mooncake fleet apply` against a Windows host, which
turns "Windows + WSL" boxes from one-managed-half into two fleet peers on
the same physical machine. Companion to spec-43 (transport) and spec-36
(Windows binary support).

---

## Problem

`mooncake.exe apply -c ...` works on Windows today (spec-36 shipped). The
fleet path doesn't, because `mooncake agentd` is unix-socket-mandatory:

- `internal/agentd/server.go:Serve` always calls `net.Listen("unix", ...)`
  before checking `cfg.BindAddr`. Even with `--bind 0.0.0.0:7879` the
  daemon crashes during the unix-listener step on Windows.
- `internal/agentd/config.go:Default()` picks XDG paths
  (`$XDG_STATE_HOME`, `/run/user/$UID`) and falls back to
  `~/.local/state`. On Windows those resolve oddly and the socket path
  becomes `/tmp/mooncake-<uid>/...` which is not a Windows-valid path.

The downstream pain: a "main_pc" machine (Windows host + WSL Ubuntu)
today has only one fleet peer (the WSL agentd). Windows-side updates
still require running `mooncake.exe apply -c platforms/windows/bootstrap.yml`
manually from an Administrator PowerShell. The Windows half can't
participate in `mooncake fleet apply` or `mooncake fleet status`.

---

## Goals

- **G1** `mooncake agentd` starts on Windows with sane defaults under
  `%LOCALAPPDATA%\Mooncake\` and `%APPDATA%\Mooncake\`.
- **G2** Daemon can run TCP-only (no unix socket) on any platform — useful
  on Windows where AF_UNIX is supported but not the default, and on
  containers/CI where there's no XDG runtime dir.
- **G3** Unix-platform defaults and behavior **unchanged**. No XDG-mode
  regressions. Existing tests pass without edits.
- **G4** The fleet controller in WSL can drive a Windows-host agentd over
  TCP+bearer just like it drives the WSL agentd today — same wire
  protocol, same `PUT /v1/files`, same `POST /v1/runs`, same SSE.

### Non-goals

- **Windows Service registration**. Task Scheduler at boot is the v1
  pattern (matches `WSL2-SSH-Autostart` in dotfiles). Service install
  helper is a follow-up.
- **NSIS/MSI installer**. `Copy-Item` from a checked-out dotfiles repo
  is enough.
- **`mooncake fleet bootstrap` over WinRM/SSH**. First-time setup is
  manual: `mooncake.exe apply -c bootstrap.yml` from Admin PowerShell.
- **TLS** on the TCP listener. LAN + bearer like the existing
  WSL-side agentd. Same trust model.

---

## Architectural decision: AF_UNIX or TCP-only on Windows?

Windows 10 1803+ supports AF_UNIX, and Go's `net.Listen("unix", path)`
works there since Go 1.18. So we have two viable shapes:

| Choice | What changes |
|---|---|
| **A.** Use AF_UNIX everywhere; just pick Windows-friendly paths. | New `config_windows.go` with `%LOCALAPPDATA%\Mooncake\agentd.sock`. Server.go unchanged. Local CLI clients (cmd/runs.go, MCP) unchanged. |
| **B.** Skip unix on Windows; TCP-only with bearer for local CLI too. | Every local CLI client gains a "if windows, dial TCP" branch. Larger surface, two paths. |

**Choice picked: A.** Smaller diff, fewer code paths to maintain, and Go's
AF_UNIX support on Win10/11 is well-tested. On Windows the socket file
sits in a user-private dir (`%LOCALAPPDATA%`) which already has
appropriate ACLs from Windows' default folder permissions — the
`os.Chmod(0o600)` step is a no-op on Windows (Go documents this), so we
build-tag it out cleanly.

**Bonus payoff:** making the unix listener *conditional* on `SocketPath != ""`
costs almost nothing extra and gives us a TCP-only mode useful in
containers, CI, and any environment where you don't want a socket file
on disk.

---

## Reuse map

This is wiring. Nothing new on the wire.

**Reused as-is:**

| File | Behavior |
|---|---|
| `internal/agentd/server.go:routes()` | Same mux serves unix + TCP. |
| `internal/agentd/middleware.go:bearerAuthMiddleware` | TCP listener guard; unaffected. |
| `internal/agentd/store.go`, `worker.go`, `sse_hub.go`, `jsonl_sink.go`, `files_handler.go` | Platform-neutral. |
| Fleet controller code in `internal/fleet/` | Already platform-neutral. |

**Extended:**

| Where | Change |
|---|---|
| `internal/agentd/config.go` | Split platform-default helpers out; `Default()` now delegates. `Validate()` accepts SocketPath="" when BindAddr is set. `EnsureDirs()` skips the socket-dir step when SocketPath="". |
| `internal/agentd/server.go:Serve` | Wrap unix-listener block in `if cfg.SocketPath != ""`. Skip `os.Chmod` on Windows. Adjust `shutdown()` and `claimSocket()` to no-op when SocketPath="". |
| `internal/agentd/worker.go:Submit` | Pre-create the run's SSE hub before pushing to the worker queue. **Fixes a Linux-masked race** where a fast controller's `GET /v1/runs/{id}/events` between submit and worker pickup found `GetHub(id)==nil` and the handler bailed silently. Exposed on Windows where worker latency is hundreds of ms. |
| `internal/agentd/runs_handler.go:streamJSONL` | Treat missing `events.jsonl` as "no replay needed" (return nil) instead of erroring out. **Fixes a related race**: handler used to return early without ever tailing the hub when JSONL hadn't been created yet. |

**New:**

| Component | Location |
|---|---|
| Unix-platform config defaults | `internal/agentd/config_unix.go` (`//go:build !windows`) |
| Windows config defaults | `internal/agentd/config_windows.go` (`//go:build windows`) |
| TCP-only mode tests | `internal/agentd/server_test.go` (additions) |

---

## Wire shape

**Unchanged.** Same endpoints, same auth model. A Windows agentd
advertises itself via `GET /v1/version` exactly like a Linux one:

```json
{
  "version": "...",
  "hostname": "DESKTOP-R809R54",
  "synced_root": "C:\\Users\\aleh\\AppData\\Local\\Mooncake\\agentd\\synced",
  ...
}
```

`SyncedRoot` is the only place the controller sees a Windows-style path,
and the apply.go code already calls `filepath.ToSlash(planRel)` so the
peer-side join with `SyncedRoot` produces a valid Windows path on the
daemon (`<root>/<scope>/...`) regardless of the controller's OS.

---

## Defaults

### Windows (per-user)

| Knob | Value |
|---|---|
| SocketPath | `%LOCALAPPDATA%\Mooncake\agentd.sock` |
| StateDir | `%LOCALAPPDATA%\Mooncake\agentd` |
| TokenPath | `%LOCALAPPDATA%\Mooncake\agentd.token` |
| BindAddr | Unset by default. CLI may default to `127.0.0.1:<port>` when invoked without `--bind` and without `--socket`. See `cmd/agentd.go` task. |

`%LOCALAPPDATA%` (not `%APPDATA%`) so the state isn't synced into
roaming profiles on AD-joined machines. Mooncake state is host-specific.

### Unix (unchanged)

XDG-as-today. Existing behavior preserved.

### System mode

Out of scope for v1 on Windows. The unix-side `--system` defaults to
`/run/mooncake/agentd.sock` and `/var/lib/mooncake/...`; the Windows
equivalent (under `%ProgramData%`) ships when someone needs it.

---

## CLI changes

```
$ mooncake agentd
  ...existing flags...
  --socket <path>     Unix socket path. Empty disables the unix listener
                      (TCP-only mode). Default: platform-specific.
  --bind <addr>       TCP listener (e.g. 0.0.0.0:7879). Required when
                      --socket is empty.
```

Validation: at least one of `--socket` / `--bind` must be configured.

On Windows, when invoked with neither `--socket` nor `--bind`, the CLI
defaults to `--bind 127.0.0.1:7878` (loopback only). LAN exposure
(`--bind 0.0.0.0:7879`) is explicit, so first-time launches don't
unexpectedly accept inbound connections.

---

## Tasks

### Task 1 — Split config defaults by build tag

1. **New** `internal/agentd/config_unix.go` with `//go:build !windows`:
   moves the bodies of `userSocketDir`, `userStateDir`, `userConfigDir`
   (XDG-aware) out of `config.go`.
2. **New** `internal/agentd/config_windows.go` with `//go:build windows`:
   - `userStateDir()` returns `%LOCALAPPDATA%`.
   - `userConfigDir()` returns `%LOCALAPPDATA%` (NOT `%APPDATA%` — keep
     state local-only).
   - `userSocketDir()` returns `%LOCALAPPDATA%`.
3. **Edit** `config.go:Default()`:
   - The user-mode socket joins `socketDir + "Mooncake" + "agentd.sock"`.
   - All three dirs join via `filepath.Join`, so on Windows you get
     `C:\Users\aleh\AppData\Local\Mooncake\agentd.sock` naturally.
   - No platform conditional in `config.go` itself — the three helpers
     hide the difference.
4. **Edit** `config.go:Validate()`:
   - Allow `SocketPath == ""` when `BindAddr != ""`.
   - Reject the case where both are empty.
5. **Edit** `config.go:EnsureDirs()`:
   - Skip the `socketDir` MkdirAll when SocketPath is empty.

### Task 2 — Conditional unix listener in server.go

1. **Edit** `server.go:Serve`:
   - Wrap the existing unix-listener block in `if s.cfg.SocketPath != "" { ... }`.
   - Same for the `os.Chmod` call. (On Windows it's a no-op anyway, but
     skipping it is the honest signal.)
   - Adjust `errCh` capacity / goroutine count: if SocketPath is empty
     skip the unix-serve goroutine.
2. **Edit** `server.go:shutdown`:
   - `s.unixSrv` may be nil; the existing nil-check already covers it.
   - Skip `os.Remove(s.cfg.SocketPath)` when SocketPath is empty.
3. **Edit** `server.go:claimSocket`:
   - Return nil immediately when SocketPath is empty.

### Task 3 — CLI defaults in cmd/agentd.go

1. **Edit** `cmd/agentd.go`:
   - When `--socket` is unset on Windows AND `--bind` is unset: default
     to `--bind 127.0.0.1:7878` and `--socket ""`. (Explicit nil-string;
     not the platform default.)
   - When `--socket` is the empty string OR unset on Windows, the unix
     listener is disabled — log this honestly at startup.
   - Help text update for `--socket` to mention the TCP-only mode.

### Task 4 — Tests

1. **`internal/agentd/server_test.go` additions**: a `TestServe_TCPOnly`
   that boots agentd with `SocketPath=""`, `BindAddr="127.0.0.1:<random>"`,
   sends one `GET /v1/version` request, and asserts both `200 OK` and
   that no socket file appears on disk.
2. **`internal/agentd/config_test.go` additions**:
   - `Validate` accepts SocketPath="" + BindAddr set.
   - `Validate` rejects both empty.
   - `EnsureDirs` doesn't try to MkdirAll a `.`-rooted socket dir when
     SocketPath is empty.
3. **Cross-compile sanity**: `GOOS=windows go build ./...` in CI (or as
   a manual check before merge).

---

## File layout (user-visible) after this spec

### Unix host (unchanged)

```
~/.config/mooncake/
  agentd.token
  controller_id
  peers.toml
~/.local/state/mooncake/agentd/
  runs/<run_id>/...
  synced/<scope>/...
/run/user/<uid>/mooncake/agentd.sock
```

### Windows host (new)

```
%LOCALAPPDATA%\Mooncake\
  agentd.sock                  ← when unix mode enabled
  agentd.token
  controller_id                ← if this box also runs as a controller
  peers.toml                   ← if this box also runs as a controller
  agentd\
    runs\<run_id>\...
    synced\<scope>\...
```

---

## Bootstrap order (manual, one-time)

```
PS Admin> mooncake.exe apply ^
            -c platforms\windows\bootstrap.yml ^
            -v variables.yml -v vars\main_pc.yml
# Bootstrap installs mooncake.exe + agentd Task + firewall rule + token

PS> wsl --shutdown && wsl
WSL> # token generated by Windows agentd is fetched manually
WSL> # or surfaced through `mooncake fleet pair main_pc-win:7879 --token-via stdin`
WSL> mooncake fleet apply main_pc        # WSL-side updates
WSL> mooncake fleet apply main_pc-win    # Windows-side updates
```

When spec-48 (tag selectors) lands:

```
WSL> mooncake fleet apply --tag host=main_pc <plan.yml>
```

Updates both halves of the same machine in parallel, multiplexed.

---

## Open questions

1. **`%LOCALAPPDATA%` vs `%APPDATA%`.** `%LOCALAPPDATA%` keeps state
   host-local (good for run history; bad if user wants their token
   roamed across machines). Going `%LOCALAPPDATA%` — token portability
   is a non-goal.
2. **System-mode on Windows.** Out of scope. `%ProgramData%\Mooncake\`
   when needed.
3. **Service vs Scheduled Task.** Scheduled Task wins for v1
   simplicity; matches the dotfiles `WSL2-SSH-Autostart` pattern. A
   proper Windows Service install command is a separate spec.
4. **AF_UNIX path conventions on Windows.** Go's `net.Listen("unix",
   path)` works with Windows paths (`C:\...`). No special handling.
   The socket file *is* a real filesystem entity; Windows Explorer
   shows it as a 0-byte file with type "File". Cosmetic, not a problem.

---

## Cross-links

- [Spec 36 — Windows support](spec-36-windows-support.md): the
  prerequisite that made `mooncake.exe apply` work.
- [Spec 43 — Fleet transport + file sync](personal-fleet/spec-43-fleet-transport-and-sync.md):
  defines `PUT /v1/files`, the TCP listener, and the bearer-auth model
  this spec extends to Windows.
- [Personal-fleet implementation order](personal-fleet/implementation-order.md):
  this spec is parallel to PR6-14, not part of the sequence.
