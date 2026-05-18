# Spec 70: Local `agentd bootstrap` — extract install primitives, add no-SSH path

**Epic:** Personal Fleet — extends spec-44's bootstrap flow and the
user-mode install added in the `--user` patch (cmd/fleet.go,
internal/fleet/bootstrap.go, internal/fleet/installer.go, and the
embedded `init/mooncake-agentd.user.service` template).
**Status:** Draft
**Effort:** M (~3–5 days)
**Value:** Medium-high — removes the SSH-to-self awkwardness when
bootstrapping the box you're sitting on, and folds two-mode install
logic (user vs system) behind a single abstraction so the next
template/flag change touches one place instead of two.
**Depends on:** the `--user` patch already in tree (Installer.AsUser,
BinaryInstallPath/TokenFilePath helpers, mooncake-agentd.user.service
template, enableLinger helper).

---

## Problem

`mooncake fleet bootstrap` is the only way to install agentd today. It
is SSH-shaped end-to-end: the binary is SCP'd, every install step
shells through `transport.Session.Run`, and the orchestrator hardcodes
"there is a remote, there is a sudoer wrapping each command." That
shape is correct for the cross-machine controller-side flow it was
designed for, but it makes "install agentd on the box I'm already
logged into" awkward:

1. **Self-loopback SSH.** Bootstrapping the local box means
   `mooncake fleet bootstrap aleh@localhost --port 2222 --user`,
   which requires an OpenSSH daemon on the local box, key auth
   working against your own user, and the SSH handshake/SCP overhead
   for a copy that could be a single `os.Rename`.
2. **Asymmetric `fleet apply`.** When the controller is also the
   target (single-host setups, dev loops, the dotfiles `mac`
   machine), the operator wants `mooncake apply -c <plan>` locally
   *and* the daemon on the same box to be installed via mooncake —
   today the daemon install requires either a second machine or the
   SSH-to-self dance above.
3. **Duplicated install logic risk.** The natural workaround is for
   a separate `mooncake agentd install` command to reimplement
   "render unit, write to UnitPath, enable+start, read token." That
   path is one template-knob away from drifting from the SSH path
   (see: the system-mode unit's `ReadWritePaths=/usr/local/bin` line
   that issue-19 added — a hand-rolled local installer that misses
   that line silently re-introduces the EROFS bug).

The `--user` patch made (3) worse, not better: every `Installer`
method now has a user-vs-system branch, and the install steps in
`bootstrap.go` mix that branching with SSH I/O. A future template
change (e.g. add a new `Environment=` directive, change the unit
location, swap the linger probe) has to be made consistently across
both the system and user branches in `installer.go` *and* across the
SSH I/O sites in `bootstrap.go`. Two flavors of "where" × two flavors
of "how" = four code paths to keep in sync.

The fix is the standard one: separate **what to install** (rendered
unit body, target paths, enable/disable commands — already on
`Installer`) from **how to put it there** (SFTP+sudo vs local
copy+sudo). Once that separation is in place, the local-bootstrap
command is a thin wrapper that picks the local "how" and reuses every
"what" the SSH path uses.

---

## Goals

- **G1** A new `mooncake agentd bootstrap` subcommand that installs
  agentd on the local machine — binary, unit, token, enable+start
  — without an SSH detour. Flags: `--user`, `--port` (default
  7878), `--bind` (default `0.0.0.0:<port>`), `--binary` (default
  `os.Executable()`), `--upgrade` (replace mismatched-version install).
- **G2** The shared install logic (render, place, enable, linger,
  detect-existing, read-token) lives in one place and is called by
  both `fleet bootstrap` (SSH) and `agentd bootstrap` (local). A
  template change is one edit, not two.
- **G3** `fleet bootstrap` continues to work bit-for-bit identically.
  Peer protocol, peers.toml shape, log lines, error messages, exit
  codes — all unchanged. The refactor is invisible to controller-side
  callers and to existing fleet plans.
- **G4** `agentd bootstrap` is idempotent on the same axes
  `fleet bootstrap` is: same version + active service → no-op (just
  prints the token). Different version → refuses without `--upgrade`.
  Missing → full install.
- **G5** `agentd bootstrap` does **not** write peers.toml. The local
  agentd doesn't know what controller will eventually pair with it;
  pairing is the controller's job (`fleet pair`). The command prints
  the bearer token + a copy-pasteable `mooncake fleet pair
  <host>:<port>` one-liner.

**Non-goals:**

- **Daemon-side changes.** This spec doesn't touch
  `internal/agentd/`. The bootstrap orchestrator and the daemon
  config/state code are independent layers.
- **Windows in v1.** The Windows install path goes through
  `bootstrapWindows` + `winutil` and has its own scheduled-task
  shape; local Windows bootstrap (no SSH) is feasible but the
  Task Scheduler S4U principal lookup is non-trivial and orthogonal
  to the user/system Linux split this spec untangles. Track as a
  follow-up.
- **Auto-discovery of an SSH-to-self loop.** If the operator runs
  `fleet bootstrap aleh@localhost`, we don't try to be clever and
  redirect to the local path. The two commands are distinct verbs;
  the operator picks.
- **A `mooncake agentd uninstall` companion.** Tracked separately.
  v1 is install-only; teardown today is the same `systemctl --user
  disable --now mooncake-agentd` the operator runs manually.

---

## Reuse map

**Reused:**

- `internal/fleet/installer.go` — `Installer` struct, `UnitPath`,
  `UnitName`, `BinaryInstallPath`, `TokenFilePath`, `Render`,
  `EnableStartCmd`, `IsActiveCmd`, `StopDisableCmd`, the two embedded
  templates (`init/mooncake-agentd.service`, `init/mooncake-agentd.user.service`).
  No changes to *what* this file describes — only its *callers* shift.
- `internal/fleet/bootstrap.go` — `existingInstall` (parses
  `mooncake --version`), `parseVersion`, `enableLinger` (idempotent
  linger probe + enable), `readToken` (cat from token file path).
  These move into the shared package as platform-independent helpers
  parameterized over an executor (see Design).
- `cmd/agentd.go` — existing `--system`, `--token-file`, `--bind`
  flags + `agentd.Default` config resolution. The new
  `agentdBootstrapCommand()` lives in this file (keeps agentd-shaped
  verbs together).
- `os.Executable()` for the default `--binary` value. Same
  default as `fleet.EnsureLocalBinaryPath()` in
  `internal/fleet/bootstrap.go:427`.

**Replaced / extracted:**

- `installBinary`, `installService`, `startAndVerify` in
  `internal/fleet/bootstrap.go` become methods/functions that take
  an `Executor` interface (see Design §1) instead of `*transport.Session
  + *sudoer` directly. The SSH-shaped helpers stay as thin adapters.

---

## Design

### 1. The Executor abstraction

Introduce a small interface in a new file
`internal/fleet/install/executor.go`:

```go
package install

import (
    "context"
    "io/fs"
)

// Executor runs install primitives against a target — local
// (os/exec) or remote (SSH session). Every method may be no-op
// idempotent at the call site, but the Executor itself does not
// enforce idempotency; that's the caller's job (existingInstall,
// EnableLinger probe, etc.).
type Executor interface {
    // Run executes cmd via `sh -c cmd` semantics on the target.
    // stdout/stderr captured, exitCode is the process exit, err
    // is non-nil only on transport / spawn failures (not on
    // non-zero exit). Mirrors transport.Session.Run.
    Run(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error)

    // WriteFile writes data to path on the target with the given
    // mode. Atomic at the call site is the caller's job (write
    // tmp + rename); WriteFile may stream straight to path.
    WriteFile(ctx context.Context, path string, data []byte, mode fs.FileMode) error

    // CopyLocalFile uploads (SSH) or copies (local) a file from
    // the controller's filesystem to a path on the target. Used
    // for the binary upload step.
    CopyLocalFile(ctx context.Context, src, dest string, mode fs.FileMode) error
}
```

Two implementations:

- `internal/fleet/install/local_exec.go` — `LocalExecutor` uses
  `os/exec.CommandContext("sh", "-c", cmd)`, `os.WriteFile`,
  `io.Copy`. No-op on hostname (it's localhost).
- `internal/fleet/install/ssh_exec.go` — `SSHExecutor` wraps
  `*transport.Session` and delegates Run/WriteFile/Upload to the
  existing session methods. This is the existing behavior, just
  routed through the interface.

The `sudoer` from `bootstrap.go` becomes a *decorator* over an
`Executor`:

```go
// Sudoer wraps an Executor's Run to prefix with `sudo -n sh -c
// '<cmd>'` when escalation is needed. isRoot=true (uid 0) and
// noSudo=true (user-mode install) both bypass the wrap.
type Sudoer struct {
    Exec   Executor
    IsRoot bool
    NoSudo bool
}

func (s *Sudoer) Run(ctx context.Context, cmd string) (stdout, stderr string, code int, err error)
```

The WriteFile / CopyLocalFile methods don't need sudo wrapping — the
caller stages into a sudo-writable path (`/tmp/...`) and then uses
`Sudoer.Run("mv -f ...")` for the final placement, same as today.

### 2. Install primitives as `Installer` methods

Extract from `bootstrap.go` into `installer.go` (or a new
`internal/fleet/install/steps.go`):

```go
// PlaceBinary copies a local binary into BinaryInstallPath via
// Executor, mkdir -p'ing the parent dir, sudo'd when needed.
// Two-stage (write tmp, then mv) so an interrupted op never leaves
// a partial binary at the canonical path.
func (i Installer) PlaceBinary(ctx context.Context, sud *Sudoer, localPath string) error

// PlaceUnit renders the unit template and writes it to UnitPath
// via Executor, sudo'd when needed. Two-stage same as PlaceBinary.
func (i Installer) PlaceUnit(ctx context.Context, sud *Sudoer) error

// EnableAndStart runs EnableStartCmd via sudoer and waits for
// /v1/version to be reachable. host is the dial target for the
// reachability poll (the local addr for LocalExecutor, the SSH
// target host for SSHExecutor).
func (i Installer) EnableAndStart(ctx context.Context, sud *Sudoer, host string) error

// EnableLinger is the idempotent loginctl probe-and-enable that
// today lives in bootstrap.go. Moves verbatim; takes Executor not
// *transport.Session.
func EnableLinger(ctx context.Context, exec Executor) error

// ExistingInstall probes BinaryInstallPath and IsActiveCmd to
// detect what's already installed. Returns ("", false, nil) when
// no install exists.
func (i Installer) ExistingInstall(ctx context.Context, exec Executor) (version string, active bool, err error)

// ReadToken reads from TokenFilePath via sudoer (system mode
// needs sudo, user mode doesn't because Sudoer.NoSudo bypasses).
func (i Installer) ReadToken(ctx context.Context, sud *Sudoer) (string, error)
```

The orchestration shape — "step 3, step 4, step 5, etc." with the
idempotency short-circuit and the version-mismatch check — moves to
a new `Installer.Bootstrap` method:

```go
// BootstrapOptions is the install-mode-and-port-and-binary input;
// strictly smaller than fleet.BootstrapOptions (no SSH target, no
// peers.toml concerns).
type BootstrapOptions struct {
    Port              int
    AsUser            bool
    LocalBinary       string
    ControllerVersion string
    Upgrade           bool
    Writer            io.Writer
    // ReachableHost is the host:port substring used for the
    // post-start /v1/version reachability probe. SSH callers pass
    // the SSH target's hostname; local callers pass "127.0.0.1".
    ReachableHost string
}

type BootstrapResult struct {
    Token     string
    OS        string
    Arch      string
    AlreadyOK bool
}

// Bootstrap runs the spec-44 §88 install sequence against the
// given Executor. Idempotent on same-version + active.
func Bootstrap(ctx context.Context, exec Executor, opts BootstrapOptions) (BootstrapResult, error)
```

`fleet.Bootstrap` becomes a thin shim: connect SSH → wrap in
`SSHExecutor` → call `install.Bootstrap` → upsert peers.toml from
the returned `BootstrapResult`. Windows path is left alone (its
`bootstrapWindows` helper has a different shape and is out of v1
scope).

### 3. The new `agentd bootstrap` command

`cmd/agentd.go` gains:

```go
func agentdBootstrapCommand() *cli.Command {
    return &cli.Command{
        Name:  "bootstrap",
        Usage: "Install agentd on the local machine (no SSH detour)",
        Description: "Install the systemd unit (Linux) or launchd " +
            "plist (macOS) for the running mooncake binary, enable + " +
            "start it, and print the bearer token + a `fleet pair` " +
            "one-liner for the controller. Use `mooncake fleet bootstrap " +
            "user@host` when the target is a different machine.",
        Flags: []cli.Flag{
            &cli.BoolFlag{Name: "user", Usage: "Linux only: install as user-scope systemd unit (default: system-scope)."},
            &cli.IntFlag{Name: "port", Value: 7878, Usage: "agentd TCP bind port"},
            &cli.StringFlag{Name: "bind", Usage: "TCP bind address override (default: 0.0.0.0:<port>)"},
            &cli.StringFlag{Name: "binary", Usage: "mooncake binary to install (default: this process)"},
            &cli.BoolFlag{Name: "upgrade", Usage: "Replace mismatched-version install"},
        },
        Action: agentdBootstrapAction,
    }
}
```

Wire it into `agentdCommand()` as a subcommand. The action detects
local OS/arch, builds the `Installer`, wraps a `LocalExecutor` in a
`Sudoer` (`IsRoot=os.Geteuid()==0`, `NoSudo=opts.AsUser`), and calls
`install.Bootstrap`.

Output on success:

```
✓ agentd installed at ~/.local/bin/mooncake (user-mode)
✓ unit at ~/.config/systemd/user/mooncake-agentd.service
✓ linger enabled for aleh
✓ agentd reachable at 0.0.0.0:7878

bearer token:
  <token>

pair from the controller:
  mooncake fleet pair main-pc.local:7878 --token-via stdin <<<'<token>'
```

The pair hint uses `--token-via stdin` (not `literal:`) so the
copy-paste doesn't accidentally land in shell history.

### 4. Subcommand registration

`agentdCommand()` today exposes the `agentd` verb as a leaf command
(it has an Action, no Subcommands). To add `bootstrap` as a sibling
verb without losing the bare `mooncake agentd` form, two options:

- **A**: Move the current action to `agentdCommand().Subcommands` as
  `run` (or default subcommand pattern via urfave/cli's
  `DefaultCommand`), add `bootstrap` alongside.
- **B**: Keep `mooncake agentd` as the daemon-run command and add
  `mooncake agentd-bootstrap` as a top-level verb.

**Pick A.** The leaf-vs-tree distinction is a small ergonomic
regression (`mooncake agentd` alone now needs a subcommand) but
keeps the verb namespace clean. Look at how
`fleetCommand()` is structured for the pattern — `Subcommands: []`
on the parent, no `Action` on the parent.

This is the only externally-visible breaking change in the spec.
Note it in `docs-next/about/changelog.md` if such a file exists, or
add a release note for the next tagged version.

---

## Test plan

### Unit (Go)

- `internal/fleet/install/local_exec_test.go` — `LocalExecutor`
  Run/WriteFile/CopyLocalFile against tempdirs. Cover: command exit
  code propagation, stderr capture, mode bits on WriteFile, partial
  write left-behind on CopyLocalFile error.
- `internal/fleet/install/ssh_exec_test.go` — `SSHExecutor` adapter
  routes through a fake `transport.Session`. Verify the wire calls
  are equivalent to today's `bootstrap.go`.
- `internal/fleet/install/steps_test.go` — `Installer.PlaceBinary` /
  `PlaceUnit` / `EnableLinger` / `ReadToken` against a fake Executor.
  Verify each step issues the expected shell commands in the
  expected order, with the user-vs-system path swap.
- `internal/fleet/install/bootstrap_test.go` — `install.Bootstrap`
  end-to-end with a fake Executor: covers idempotent short-circuit,
  version-mismatch refusal, upgrade override, linger-already-on
  short-circuit.
- `cmd/agentd_bootstrap_test.go` — `agentdBootstrapAction` against a
  tempdir-rooted `LocalExecutor`. Skip on Windows (out of scope).
  Cover: full install, idempotent re-run, `--upgrade` flow,
  `--user` mode, output contains the pair hint with the right port.

### Integration

- `fleet bootstrap` test fixtures continue to pass unchanged — the
  refactor is a pure code move from the call site's perspective.
- New integration test: spawn a real local agentd via
  `agentd bootstrap` in a tempdir-rooted XDG env, verify
  `/v1/version` is reachable, verify `mooncake fleet pair` against
  it succeeds and writes peers.toml.

### Manual verification (consumer)

- On the dotfiles-side main_pc box, after this spec ships:
  `mooncake agentd bootstrap --user --port 7878 --upgrade`. Confirm
  the resulting agent is identical (same unit body, same paths) to
  what `fleet bootstrap aleh@localhost --user` would have produced.

---

## Migration

- The refactor is internal — no peer-protocol changes, no
  peers.toml shape changes, no daemon config changes.
- `fleet bootstrap` callers see no difference. The exit codes,
  error messages, and `[peer] step N: ...` log prefixes from
  `bootstrap.go` are preserved by routing them through the same
  `report` closure (now passed to `install.Bootstrap` via
  `BootstrapOptions.Writer`).
- The `agentd` → `agentd run` subcommand split (Design §4) is the
  only visible break. Existing CLI users who type `mooncake agentd`
  with no subcommand will hit a "missing subcommand" error;
  surface a helpful message via urfave/cli's `Before` hook or a
  custom `OnUsageError`. Pre-spec scripts that invoke the daemon
  directly (e.g. systemd ExecStart in older units) need to switch
  to `mooncake agentd run`. The embedded templates in this repo
  already get updated as part of the spec; out-of-tree
  consumers (mostly dotfiles' historical hand-rolled units) are
  rare and surface a clear error on next start.

---

## Open questions

1. **Should `agentd bootstrap` also handle macOS user-mode?** Today
   the Linux user-mode work in the `--user` patch has no darwin
   equivalent. macOS LaunchAgents (vs LaunchDaemons) are the natural
   fit but the `Installer.UnitPath` / `EnableStartCmd` for
   `os == "darwin" && AsUser` aren't wired. Probably a follow-up
   spec; flag the gap with an "unsupported on darwin" error from
   `agentd bootstrap --user` for now.
2. **Token printout via stdout vs a designated file?** Stdout is the
   pair-hint shape, but it lands in shell history if the operator
   copies the whole `mooncake agentd bootstrap` invocation+output.
   Alternative: write to a 0600 file and print the path. v1 prints
   stdout; revisit if it bites.
3. **`agentd bootstrap` on a box where `fleet bootstrap` already
   ran.** The idempotency check (same version + active) should
   work uniformly — `ExistingInstall` is now agnostic to who put
   the install there. Worth a manual test, not a blocker.

---

## Out-of-scope follow-ups

- `mooncake agentd uninstall` (stop + disable + remove unit + remove
  token).
- Windows local bootstrap (Task Scheduler S4U principal lookup
  without an SSH session's `$env:USERNAME`).
- Auto-pair: `agentd bootstrap --pair-with <controller-host>` that
  SSHes into the controller and runs `fleet pair` for the operator.
  Convenient but inverts the trust direction (the box being installed
  initiates a connection back to the controller); needs a separate
  threat-model pass.

---

## Pickup checklist (for the implementing agent)

1. Read `internal/fleet/installer.go` and `internal/fleet/bootstrap.go`
   end-to-end. The `--user` patch landed recently — the dual-mode
   branching is where the refactor pays off.
2. Land Phase 1 (Executor interface + adapters) first; verify the
   existing fleet-bootstrap tests still pass with no behavior
   change.
3. Phase 2 (extract install primitives to `Installer` methods +
   `install.Bootstrap`). At each step, the SSH path goes through
   the new abstraction; tests stay green.
4. Phase 3 (`agentd bootstrap` command + agentd subcommand split).
   New tests cover the local path. Manual smoke-test against the
   dotfiles main_pc box (the consumer that motivated the spec).
5. Update `docs-next/` if there's user-facing CLI documentation;
   add a release note for the `agentd run` subcommand split.

Claim spec-70 in `~/.mooncake/claims.jsonl` before starting (per
mooncake's `CLAUDE.md`). Worktree: `git worktree add ../mooncake-spec-70
-b worktree-spec-70`.
