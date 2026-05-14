# Spec 40: SSH Bootstrap Transport

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md), sub-epic P2.
**Status:** Draft
**Effort:** M (~3–5 days)
**Value:** High — eliminates the bootstrap-step manual work. A fresh box
joins the fleet in one command from the controller. After this spec, P5
(`mooncake fleet bootstrap`) is just the UX wrapper.
**Depends on:** spec-39 (peer transport + peers.toml) for the post-install
hand-off.

---

## Problem

After spec-39 ships, agentd-managed peers work great — but each peer has to
be set up by hand once: install mooncake, write a systemd / launchd unit,
start the daemon, read the auto-generated token, paste it into the
controller's `peers.toml`. That's the kind of step that kills a "5-minute
demo".

We want one command from the controller:

```
$ mooncake fleet bootstrap user@new-box
```

…to drive the whole sequence over SSH and end with the new box appearing in
`peers.toml`.

SSH is the right transport for this one task because it sidesteps the
chicken-and-egg of "you can't agentd-transport to a box that doesn't have
agentd." It's NOT the everyday transport — that remains spec-39's
agentd+HTTP flow.

---

## Goals

- **G1** A controller-side SSH driver that connects to a target as
  `user@host`, detects OS/arch, and runs an idempotent install sequence.
- **G2** Cross-platform: macOS arm64/x86_64 and Linux x86_64/arm64. (Windows
  deferred.)
- **G3** Pull the binary from the controller-side filesystem (the one running
  the bootstrap) and `scp` it to the target. No external download required.
- **G4** Idempotent: re-running against an already-bootstrapped host is a
  no-op (or refreshes config files only).
- **G5** Final step: read the peer's freshly-minted bearer token back over
  SSH and append a new `[[peers]]` entry to the controller's `peers.toml`.

**Non-goals:**

- General-purpose "fleet apply over SSH". The escape hatch from the epic
  (`--via-ssh` on apply) is explicitly **out of scope** here — it's a
  separate feature and a separate spec if we ever ship it.
- Custom binary distribution (URL fetch, version selectors). v1 always uses
  the controller's local binary.
- Windows / freebsd / other-platform installers.
- Privilege escalation negotiation. The user must already be able to
  `ssh user@host` and `sudo` or be root.
- Uninstall. (Could be a `mooncake fleet decommission` later.)

---

## Reuse map

**Reused:**

- `peers.toml` format and loader from spec-39.
- The agentd binary itself — no changes to the daemon for this spec.
- Existing systemd unit and launchd plist templates if any exist; otherwise
  authored here.

**New:**

| Component | Location |
|---|---|
| SSH driver wrapping `golang.org/x/crypto/ssh` | `internal/fleet/transport/ssh.go` |
| Bootstrap orchestration | `internal/fleet/bootstrap.go` |
| systemd unit template | `init/mooncake-agentd.service` |
| launchd plist template | `init/com.mooncake.agentd.plist` |
| Linux install script | `init/install-linux.sh` (or generated inline) |
| macOS install script | `init/install-darwin.sh` |
| CLI: `mooncake fleet bootstrap` | `cmd/fleet.go` (extends spec-39 scaffold) |

---

## Bootstrap sequence

```
$ mooncake fleet bootstrap aleh@macbook.lan --name macbook --tags darwin,workstation

[macbook.lan] connecting…                          ✓
[macbook.lan] detecting platform…                  darwin arm64
[macbook.lan] checking existing install…           none
[macbook.lan] uploading mooncake binary…           14.2 MiB
[macbook.lan] installing to /usr/local/bin…        ✓
[macbook.lan] writing launchd plist…               ✓
[macbook.lan] starting service…                    ✓
[macbook.lan] reading bearer token…                ✓
[macbook.lan] verifying agentd reachability…       7878/tcp ok
[macbook.lan] writing peer entry…                  ~/.config/mooncake/peers.toml
✓ macbook is now part of your fleet

Try: mooncake fleet apply config.yml
```

Each step is a discrete function with a single SSH session reused throughout
(via `golang.org/x/crypto/ssh` + sftp client).

### Step 1: Connect + auth

- Use the user's SSH agent (`SSH_AUTH_SOCK`) primarily.
- Fall back to `~/.ssh/id_ed25519` / `~/.ssh/id_rsa` if no agent.
- Honour `~/.ssh/config` for HostName, Port, User, IdentityFile.
- Host key verification against `~/.ssh/known_hosts`. Prompt on unknown host
  with a clear yes/no.
- `--known-hosts <file>` flag to override (CI use).

### Step 2: Detect platform

```sh
uname -s   # Linux / Darwin
uname -m   # x86_64 / arm64 / aarch64
```

Normalize: `(Linux|Darwin) × (amd64|arm64)`. Refuse anything else with a
clear error.

### Step 3: Check existing install

```sh
command -v mooncake && mooncake --version
systemctl is-active mooncake-agentd  # or launchctl print …
```

If a mooncake of the same version is installed and running, skip Steps 4–6.
If a different version is installed, prompt unless `--upgrade`. If installed
but not running, restart.

### Step 4: Upload binary

- Source path: `os.Executable()` on the controller (the running mooncake binary)
  unless `--binary <path>` is given.
- SFTP upload to `/tmp/mooncake.<random>` first; then `mv` into place via SSH
  (with `sudo` when needed). Avoids partial-write race.
- Final destination: `/usr/local/bin/mooncake` (mode 0755).

### Step 5: Install service unit

Linux (systemd):

```ini
# /etc/systemd/system/mooncake-agentd.service
[Unit]
Description=Mooncake host daemon
After=network.target

[Service]
Type=simple
User=mooncake
ExecStart=/usr/local/bin/mooncake agentd --system --bind 0.0.0.0:7878
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

macOS (launchd):

```xml
<!-- /Library/LaunchDaemons/com.mooncake.agentd.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
  <key>Label</key><string>com.mooncake.agentd</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/mooncake</string>
    <string>agentd</string>
    <string>--system</string>
    <string>--bind</string>
    <string>0.0.0.0:7878</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict>
</plist>
```

Templates live under `init/` and are embedded via `//go:embed`. SSH driver
writes them to the target with substituted values (port, etc.).

### Step 6: Start + verify

```sh
# Linux
systemctl daemon-reload && systemctl enable --now mooncake-agentd

# macOS
launchctl bootstrap system /Library/LaunchDaemons/com.mooncake.agentd.plist
```

Verify by polling `http://<host>:7878/v1/version` (with bearer auth) for up
to 10 seconds.

### Step 7: Read the bearer token

```sh
cat /var/lib/mooncake/agentd/agentd.token
```

(Or wherever the daemon's `TokenPath` resolved to in `--system` mode. Default
in system mode is `/etc/mooncake/agentd.token` per spec-39 conventions.)

### Step 8: Update `peers.toml`

Append on the controller:

```toml
[[peers]]
name      = "macbook"            # --name flag, defaults to ssh hostname
addr      = "macbook.lan:7878"   # ssh host + agentd port
transport = "agentd"
token     = "<read from step 7>"
tags      = ["darwin", "workstation"]  # --tags flag, optional
```

Use a TOML-aware writer (don't string-concatenate). If a `[[peers]]` with
the same `name` already exists, replace it; print a diff line.

---

## CLI

```
mooncake fleet bootstrap <user@host> [flags]
  --name <id>           Peer name in peers.toml (default: hostname from <user@host>)
  --tags <a,b,c>        Tags to associate with the peer
  --port <p>            agentd bind port on target (default 7878)
  --binary <path>       Source binary (default: controller's $0)
  --upgrade             Allow replacing a different version of mooncake
  --known-hosts <path>  Override ~/.ssh/known_hosts
  --dry-run             Show what would happen, change nothing
```

---

## Failure modes & rollback

The driver progresses step-by-step. On failure at step N:

| Failed step | Action |
|---|---|
| 1 (connect) | Print SSH error verbatim, exit. Nothing to roll back. |
| 2 (detect) | Print unsupported-platform error, exit. |
| 3 (check) | If existing install detected without `--upgrade`, print message and exit cleanly. |
| 4 (upload) | Remove `/tmp/mooncake.<random>`; exit. |
| 5 (install) | Remove `/usr/local/bin/mooncake.<random>` temp; do NOT remove an existing prior binary. Exit. |
| 6 (start) | Disable the unit / unload the plist; exit with the daemon's stderr captured. |
| 7 (read token) | Service is up but token file unreadable — print clear error pointing at the file path. Don't touch local peers.toml. |
| 8 (peers.toml write) | Print what would have been written; print the manual command to add it. Service stays installed. |

No transactional rollback across steps; document the partial state clearly in
each failure path.

---

## Tasks

### Task 1 — SSH driver skeleton

1. New `internal/fleet/transport/ssh.go`:
   - `Connect(target string, opts ConnectOptions) (*SSHSession, error)`.
   - `Run(cmd string) (stdout, stderr string, exitCode int, err error)`.
   - `Upload(localPath, remotePath string, mode os.FileMode) error` via SFTP.
   - `WriteFile(remotePath string, body []byte, mode os.FileMode) error`.
2. Auth: agent → key files → fail. No password support.
3. Known-hosts verification: use `golang.org/x/crypto/ssh/knownhosts`. On
   unknown host, prompt unless `--yes` passed.

### Task 2 — Platform detection

1. `(s *SSHSession) DetectPlatform() (os, arch string, err error)` runs the
   uname pair, normalizes to {linux,darwin} × {amd64,arm64}.

### Task 3 — Installer templates

1. `init/mooncake-agentd.service` (systemd template).
2. `init/com.mooncake.agentd.plist` (launchd template).
3. Embed both with `//go:embed`.

### Task 4 — Bootstrap orchestration

1. New `internal/fleet/bootstrap.go`. Function
   `Bootstrap(ctx, target string, opts BootstrapOptions) error` runs the
   eight steps. Each step is its own function; the orchestrator prints a
   progress line per step.
2. Idempotency: each step checks pre-state before mutating.

### Task 5 — CLI surface

1. Extend `cmd/fleet.go` with `bootstrap` subcommand.
2. Parse `user@host[:port]`. Apply flags. Resolve `--binary` default to
   `os.Executable()`.
3. On success, print the "Try: mooncake fleet apply config.yml" hint.

### Task 6 — Tests

1. Unit tests for SSH driver against a containerized OpenSSH server (no
   external mock dependencies — use Docker via `testcontainers-go` if
   available, or `dockertest`).
2. Integration test: bootstrap into an alpine container with init=tini
   running, verify `/v1/version` reachable from controller after Bootstrap
   returns.
3. Failure tests: kill the SSH session mid-upload, mid-install; assert each
   step's documented rollback runs.

---

## Open questions

1. **Which user does the daemon run as?** In system mode, ideally a
   dedicated `mooncake` user with limited privileges. v1 bootstrap creates
   it (`useradd -r -s /usr/sbin/nologin mooncake` on Linux; macOS uses the
   `_mooncake` convention). Risk: the daemon needs to read user dotfiles
   under sync targets; running as a service user breaks personal use cases.
   Trade-off open.
2. **Auto-update path.** After bootstrap, how does the peer's mooncake stay
   in sync with the controller's version? Not addressed here. Document the
   gap; resolve in a separate spec or punt to manual re-bootstrap.
3. **Token rotation.** If a peer's token leaks, we need a way to rotate it
   that doesn't require a full re-bootstrap. Out of scope here; flag for
   spec-43 or a dedicated `fleet rotate-token` spec.
