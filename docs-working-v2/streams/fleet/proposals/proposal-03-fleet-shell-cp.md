# Proposal 03: `fleet shell <peer>` + `fleet cp` — close the everyday gap that drives people back to SSH

**Status:** Draft proposal
**Effort:** S (~3 days for both; share the agentd channel)
**Value:** Medium — these aren't new capabilities (SSH works), but
they remove the friction that pulls operators back to ad-hoc SSH and
breaks the audit trail.

---

## Problem

The fleet contract is: agentd is the trusted control plane on each
peer. Every change goes through it. Audit is preserved.

The fleet CLI is: `apply`, `exec`, `observe`, `ps`, `watch`, `status`,
`doctor`, `logs`, `facts`, `bootstrap`, `pair`, `discover`, `init`,
`upgrade`. Twelve verbs.

Missing: two things every operator needs daily.

**1. Interactive shell.** When something goes wrong on a peer, the
operator wants an interactive session. Today they `ssh` — bypassing
agentd, leaving no audit trace, requiring SSH credentials that
shouldn't be necessary (mooncake has its own token).

**2. File transfer.** A common cycle: run `fleet exec` to debug → fix
a file locally → push it back. The current loop is `scp` (more SSH)
or `mooncake fleet apply` (overkill for one file).

Both are escape hatches that bypass the daemon. If they're the
"happy path" for everyday ops, the fleet contract is fiction.

## Proposal A: `mooncake fleet shell <peer>`

```bash
mooncake fleet shell main_pc
# Opens an agentd-mediated PTY. The peer's bash. The controller
# sees every command (audit). agentd records the session in its
# run history with a synthetic step type "shell.interactive".
```

Behavior:
- Establishes a WebSocket / HTTP/2 stream to `/v1/shell` on the
  peer
- agentd opens a bash (or `--shell` override) with a pty
- Bidirectional bytes
- Window-resize forwarded (TIOCGWINSZ)
- Records every line as a synthetic run event so `fleet logs <peer>`
  can replay the session later

Flags:
| Flag | Behavior |
|---|---|
| `--shell value` | Interpreter (default: the peer's `$SHELL` or `/bin/bash`) |
| `--no-record` | Don't persist the session (still streams events but doesn't write to runs.jsonl). Default false. |
| `--become` | Open the shell as root (sudo prereq same as `fleet exec --become`) |

Exit:
- Ctrl-D / `exit` closes the remote shell cleanly
- Ctrl-C in the shell sends SIGINT to the remote process
- Ctrl-Q sequence (or some escape) for "detach without killing"

Output sketch:
```
$ mooncake fleet shell main_pc
[main_pc] interactive shell as user `aleh` (run 01KRPK..., recorded)
aleh@main_pc:~$ ps aux | grep mooncake
...
aleh@main_pc:~$ exit
[main_pc] session ended (47s, 14 commands)
```

## Proposal B: `mooncake fleet cp`

Two directions:

```bash
# Upload (controller → peer)
mooncake fleet cp ./local.conf <peer>:/etc/mooncake/peer.conf

# Download (peer → controller)
mooncake fleet cp <peer>:/var/log/mooncake/last.log ./

# Multi-peer upload (broadcast a config to a tag)
mooncake fleet cp ./shared.conf --peer tag=production:/etc/shared.conf

# Multi-peer download (gather logs from a tag)
mooncake fleet cp --peer tag=production:/var/log/syslog ./logs/
   # → ./logs/main_pc-syslog, ./logs/laptop-syslog, ...
```

Behavior:
- Upload uses the existing `PUT /v1/files` endpoint that
  `fleet apply` uses to sync plan-dir, but for a single file
- Download uses `GET /v1/files?path=<>` (new endpoint)
- Subject to `--max-sync-bytes` (already configured on agentd)
- Mode + ownership preserved by default (`--no-preserve` to opt out)
- Audit: each transfer logged as a "files.transfer" event

Flags:
| Flag | Behavior |
|---|---|
| `--mode 0644` | Explicit mode (default: source mode) |
| `--owner user[:group]` | Explicit owner (default: current user on peer) |
| `--no-preserve` | Don't replicate mode/owner from source |
| `--max-size 100M` | Override per-file size cap |

## API on agentd

```
GET  /v1/files?path=<...>                 Download a file (returns bytes, X-Mode, X-Owner headers)
PUT  /v1/files?path=<...>&mode=...        Upload (mode/owner from headers)
WSS  /v1/shell                            PTY session (existing pattern as Kubernetes kubectl exec)
```

agentd authn: same bearer token. agentd authz: in v1, having a
valid token means full access. Future v2 could scope tokens (read,
write, shell, exec) but that's a separate spec.

## Receipts

From audit:
- Round 30 fleet exec testing: I wanted to `cd /tmp && ls` on a
  peer interactively. Couldn't. Had to `fleet exec "cd /tmp && ls"`,
  which works but loses interactivity.
- Multiple rounds where I wanted to `cat` a config file on the peer
  after editing locally — used `fleet exec "cat /etc/foo.conf"`
  which works for read; for write, `fleet apply` was overkill for
  one config.
- The current artifact bundle includes `events.jsonl`, `plan.json`,
  `facts.json`, `stdout.log`, `stderr.log` — adding the file in
  question requires running an entire playbook to upload it.

## Why this fits the fleet pitch

The README sells: "Drive plans across machines you own". The
fleet pitch breaks down when operators have to SSH for the
everyday stuff. If `fleet shell` and `fleet cp` exist, the entire
ops cycle stays in mooncake:

- Plan / apply → fleet apply
- Inspect → fleet status / observe / ps / facts
- Triage → fleet shell
- Patch → fleet cp + fleet exec
- Upgrade → fleet upgrade

No `ssh` required. Audit is complete. Tokens (not SSH keys) are
the access primitive.

## Risks

- **`fleet shell` introduces a long-lived stream**. agentd needs
  to handle interactive timeouts (no input for N minutes →
  prompt to detach or kill). Inherit kubectl's defaults.
- **PTY support across OS** — agentd on Linux is the primary
  target. macOS works (Unix pty). Windows requires conhost/ConPTY
  — leave as v2 (--shell powershell with non-PTY-piped IO works
  for now).
- **File transfer can blow audit log size**. `events.jsonl` should
  log the *intent* (path, size, sha256) not the bytes. Bytes go to
  the sync directory, audited separately.

## What this doesn't address

- **`fleet edit <peer>:<path>`** — download → spawn $EDITOR
  locally → upload on save. Composes from shell + cp; defer to v2.
- **Sudo-aware file operations** (cp into /etc as root) — needs
  the same sudo flow as `fleet exec --become`. Reuse the existing
  shape; document it once.
- **Concurrent shell sessions to the same peer**. Allow N, list
  via `fleet ps --status shell`.

## Sequencing with other proposals

- Lands well *after* proposal-02 (`fleet kill`), since both need
  agentd's run-lifecycle hooks
- Lands well *before* proposal-04 (global flags), since the
  shell/cp subcommands need to be in the surface to consider for
  flag promotion
