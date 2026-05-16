# Proposal 06: `fleet bootstrap --resume` + idempotent rerun — make the 8-step ritual recoverable

**Status:** Draft proposal
**Effort:** S (~2 days)
**Value:** Medium — `fleet bootstrap` is the first 30 seconds of
mooncake-on-a-new-machine. If step 6 fails (network blip, sudo
prompt timing out), the user has to manually unwind and retry.
Recoverable bootstrap = friendly bootstrap.

---

## Problem

`fleet bootstrap` runs an 8-step sequence (per the help text):

```
1. SSH to <user@host>
2. Detect platform
3. Skip 4-6 if same version installed
4. SFTP the binary; sudo-install
5. Render+install systemd / launchd / scheduled-task
6. Enable + start the service; wait for /v1/version
7. Read bearer token
8. Upsert [[peers]] entry in peers.toml
```

When step N fails, the controller knows what failed but the
peer is in a partial state:

- Step 4 fails → binary copied to `/tmp/`, no `/usr/local/bin/`
  install. Re-run starts from step 1.
- Step 5 fails → binary installed but no service unit. Re-run
  starts from step 1, redoing 4 (idempotent but wasteful).
- Step 6 fails → unit installed but daemon didn't come up.
  Maybe a port conflict? Maybe SELinux? The user has to
  diagnose AND know which step was reached AND how to clean up.
- Step 7 fails → token read failed (permissions). Daemon running
  but no peers.toml entry.

Today the user's response is: SSH in, hand-clean partial state,
retry. That's the antithesis of "Docker for AI agents" —
self-healing should be the default.

`mooncake doctor` exists for the post-install case. There's no
"diagnostic during bootstrap" surface.

## Proposal A: idempotent rerun

Make `fleet bootstrap <user@host>` safe to re-run from any
partial state. Each step:

1. **Detects** whether the desired post-condition is already met
2. **Skips** if so (logs "step N: already at target state")
3. **Performs** otherwise

Examples:
- Step 4 (binary install): check `which mooncake` on peer +
  `--version` matches expected. Already installed → skip.
- Step 5 (service unit): check `systemctl is-enabled mooncake-agentd`.
  Already enabled → skip.
- Step 6 (daemon up): check `/v1/version` reachable. Already up
  → skip.
- Step 7 (token): if `peers.toml` already has the peer with a
  token that matches `agentd.token` on the peer → skip the
  read (just confirm).

This is "every step is its own idempotent check" — the same model
the kernel uses for action handlers.

Today's step-3-skip ("skip 4-6 if same version installed") is the
half-measure; extending the skip rule to every step generalizes it.

## Proposal B: `fleet bootstrap --resume`

When `bootstrap` fails, leave a small **continuation marker** in
`~/.cache/mooncake/bootstrap-<peer>.toml`:

```toml
peer = "main_pc"
host = "192.168.1.5"
last_step = 6
last_error = "service failed to bind 7878 (port in use)"
attempted_at = "2026-05-15T20:00:00Z"
peer_state = {
  binary_path = "/usr/local/bin/mooncake",
  service_unit_installed = true,
  service_running = false,
  token_file = "/etc/mooncake/agentd.token"
}
```

Then:

```bash
$ mooncake fleet bootstrap --resume main_pc
fleet bootstrap: resuming from last_step=6 (service start)
  - Diagnosed: port 7878 in use by PID 12345 (caddy)
  - Fix options:
      1. Stop the conflicting service: ssh main_pc 'sudo systemctl stop caddy'
      2. Use a different agentd port: --agentd-port 7879
  Choose [1/2/abort]: 2
   → retrying with --agentd-port 7879
  ✓ step 6: daemon started, /v1/version reachable
  ✓ step 7: token read
  ✓ step 8: peers.toml updated
fleet bootstrap: complete (resumed at step 6)
```

The marker file is auto-deleted on successful completion.

## Proposal C: `fleet bootstrap --dry-run`

Just like `mooncake apply --dry-run`. Plan the bootstrap; don't
do it:

```
$ mooncake fleet bootstrap --dry-run user@new-box
fleet bootstrap (dry-run): user@new-box

PLAN:
  ↑ step 1: SSH connect via id_ed25519
  ↑ step 2: probe platform (expects linux/amd64 based on hostname pattern)
  ↑ step 3: check if mooncake-0.2.0 installed → expected: not installed
  ↑ step 4: SFTP /home/aleh/.../mooncake to /tmp/, sudo-install to /usr/local/bin
  ↑ step 5: render systemd unit (linux), install to /etc/systemd/system/
  ↑ step 6: systemctl enable + start
  ↑ step 7: read /etc/mooncake/agentd.token
  ↑ step 8: append [[peers]] to /home/aleh/.config/mooncake/peers.toml

  cost: ~30s wall; sudo on remote required for step 4-6
  reversible: yes (mooncake-uninstall.sh available)
  No changes made.
```

`--dry-run` is a great onboarding check. New users typing
`fleet bootstrap` for the first time can see exactly what's
about to happen.

## API

| Flag | Behavior |
|---|---|
| `fleet bootstrap <user@host>` | Idempotent bootstrap (proposal A) |
| `fleet bootstrap --dry-run <user@host>` | Plan-only (proposal C) |
| `fleet bootstrap --resume <peer-name>` | Continue from last-failure marker (proposal B) |
| `fleet bootstrap --reset <peer-name>` | Drop the marker, reset to step 1 next time |
| `fleet bootstrap --force <user@host>` | Re-do every step regardless of detected post-condition |

## Receipts

From the audit:
- Round 37: I tested `fleet bootstrap fake-user@127.0.0.1` and got
  a clean error "ssh connect: no auth methods available
  (start ssh-agent or place ~/.ssh/id_ed25519)". Great error for
  step 1. But for failures at step 6+ there's no equivalent
  resume story.
- spec-44 §88 names the 8-step sequence — the spec already
  defines the state machine; making it idempotent is the
  obvious next step.

## Why this matters for the agent story

Mooncake's pitch includes "fleet apply" working for AI-driven ops
loops. An agent calling `fleet bootstrap` against a new VM is a
likely workflow. Agents fail partial state worse than humans —
they don't have the intuition to clean up. Idempotent + resumable
bootstrap is what makes the agent flow safe.

## Implementation sketch

The 8 steps already exist; each becomes:

```go
type Step interface {
    Name() string
    DetectPostCondition(peer *Peer) (bool, error)   // already satisfied?
    Execute(peer *Peer, opts BootstrapOpts) error
    Reverse(peer *Peer) error                       // for --cleanup
}
```

The bootstrap orchestrator:
1. For each step in order:
   a. `DetectPostCondition` — if true, log "step N: already done"
      and continue
   b. else `Execute` — on success, continue; on failure, write
      marker file and exit with error
2. On success: delete marker file (if any)

`--dry-run` skips the Execute calls, calls only DetectPostCondition,
and renders the plan.

`--resume` reads the marker, fast-forwards past steps already
known to be done, retries from `last_step`.

## What this doesn't address

- **Uninstall** (`fleet uninstall <peer>`) — the reverse of
  bootstrap. Out of scope; defer to user demand.
- **Bootstrap with custom agentd config** (non-default state-dir,
  custom log level) — out of scope.
- **Cross-platform consistency** — Linux / macOS / Windows have
  different step 5 (service unit). Each platform's step 5 needs
  its own `DetectPostCondition`. spec-44 already factored this.

## Pairs with

- **proposal-02 (fleet kill)** — both need clean lifecycle hooks
  on agentd
- **proposal-05 (fleet doctor --all + last-applied)** — the
  diagnostic layer that helps when bootstrap fails mid-step
- **DX proposal-05 (error recipes)** — bootstrap errors should
  follow the doctor template
