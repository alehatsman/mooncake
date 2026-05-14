# Personal Fleet — Session Handoff

> Snapshot of state at the end of the May 14 working session. Use this
> to pick the work up cleanly on another machine.

---

## TL;DR

- The Personal Fleet epic exists end-to-end: an x1 laptop applied your
  real dotfiles to a WSL Ubuntu peer (main_pc, 192.168.1.68) in 22s
  over the LAN, no SSH tunnel.
- All work lives on branch `worktree-epic-personal-fleet`. **Push it
  before switching machines** — currently local-only.
- Mooncake is functional. The next planned step is PR6 (parallel
  multiplexer + ^C banner), or any of the remaining epic specs
  (status / logs, full bootstrap with native SSH, discovery, overlays).

---

## What's on the branch

`worktree-epic-personal-fleet`, in chronological order:

```
b862093  feat(fleet): bump SSE buffer to 16 MiB + machine-name shorthand for apply
f9d6110  feat(fleet): --plan-dir flag for fleet apply
ab48cd5  feat(fleet): minimal bootstrap + pair commands (spec-40/43 lite)
2d9264b  feat(fleet): plan-dir walker + sync + Apply orchestration (spec-39 PR5)
497ea8b  feat(fleet): peer transport client (HTTP + SSE) (spec-39 PR4)
b1f0f34  feat(fleet): CLI scaffold + peers.toml + controller_id (spec-39 PR3)
ee1a65c  feat(agentd): PUT/HEAD /v1/files with sandboxed sync root (spec-39 PR2)
d1f2142  feat(agentd): TCP listener + bearer auth + token + version fields (spec-39 PR1)
ba85017  docs(personal-fleet): draft epic, specs 39-44, and implementation order
```

**Lines added across the session:** ~7,200 (specs + docs + Go code +
tests). Tests pass: `go test ./... → 73 packages, 0 failures`.

---

## Docs you should read first when you resume

| Path | Why |
|---|---|
| `docs-working/epics/epic-personal-fleet.md` | The full epic — design decisions, architecture, scope boundary vs. enterprise fleet. |
| `docs-working/specs/personal-fleet/implementation-order.md` | The 14-PR roadmap and what's done vs. pending. |
| `docs-working/specs/personal-fleet/spec-39-fleet-transport-and-sync.md` | The core spec (PR1-5 implement this). |
| `docs-working/SESSION-HANDOFF-fleet.md` | This file. |

The other specs (40-44) are draft and reference but not yet implemented.

---

## What's done (PR-by-PR per the implementation-order doc)

- [x] **PR1** — agentd TCP listener + bearer auth + token + `/v1/version` fields
- [x] **PR2** — `PUT/HEAD /v1/files` with sandboxed sync root
- [x] **PR3** — fleet CLI scaffold + `peers.toml` + `controller_id`
- [x] **PR4** — peer transport client (HTTP + SSE) with integration test
- [x] **PR5** — plan-dir walker + sync + Apply orchestration (single peer)
- [x] **Out-of-order**: minimal `fleet bootstrap` + `fleet pair` (spec-40/43 lite)
- [x] **Out-of-order**: `--plan-dir` flag (needed for dotfiles repos with `../` imports)
- [x] **Out-of-order**: SSE buffer bumped to 16 MiB on both sides
- [x] **Out-of-order**: `mooncake fleet apply <machine>` shorthand
- [ ] **PR6** — parallel multiplexer for N peers + `^C` banner (next planned)
- [ ] **PR7** — `fleet status` + `logs` + `facts`
- [ ] **PR8** — same group as 7 (different subcommands)
- [ ] **PR9-11** — proper spec-40/43 (native SSH lib, systemd/launchd units, idempotent rollback)
- [ ] **PR12** — discovery (mDNS + ssh-config import)
- [ ] **PR13** — `fleet init` interactive flow
- [ ] **PR14** — per-host overlays + tag selectors

---

## Real-world findings from this session (capture for future iterations)

Three things broke and got fixed; all three are worth thinking about
properly in the spec polish PRs.

1. **WSL2 + mirrored networking needs two firewall rules**, not just
   one. `New-NetFirewallRule` opens the host NIC, but the WSL VM sits
   behind a separate Hyper-V Firewall — `New-NetFirewallHyperVRule`
   is required too. Captured in your dotfiles at
   `~/dotfiles/platforms/windows/bootstrap.yml` (commit `744ad2f`).
2. **Packer.nvim's `git clone --progress` output broke the 1 MiB SSE
   buffer cap** — fixed by bumping to 16 MiB. But the root cause is
   the executor batching all `\r`-separated cursor-rewrite output
   into one giant `step.stdout` event. The buffer bump is a band-aid;
   the proper fix is to split very long lines on the daemon side
   before they hit the JSONL sink. Worth a spec-polish item.
3. **Most mooncake "language toolchain" presets (python, rust, go,
   java) moved to `presets-archive/` upstream.** Your dotfiles
   `common.yml` was calling `use: python` etc. against presets that
   no longer exist in `presets/`. Now installs via shell+apt/brew/
   pacman directly (commit `8f93942` on dotfiles `main`). If you ever
   want pinned-version installs back, `mise` is the surviving preset
   — install mise first, then `mise install python@3.12.12` etc.

---

## State on disk

### On this laptop (x1)

- Worktree at `~/projects/mooncake/.claude/worktrees/epic-personal-fleet/`
  on branch `worktree-epic-personal-fleet`. **Unpushed.**
- Local mooncake binary at `~/mooncake` (compiled from this worktree).
- `~/.config/mooncake/`:
  - `agentd.token` — local agentd's bearer (44 base64url chars)
  - `controller_id` — UUIDv4 from `EnsureControllerID`
  - `peers.toml` — has one entry: `main_pc` at `192.168.1.68:7878`
- `~/.local/state/mooncake/agentd/`:
  - `runs/<id>/` — old run history
  - `synced/` — likely empty (we were never the target of a sync)
- Local agentd is **not running** right now (was up earlier for the
  multi-peer test; we cleaned up).

### On main_pc (192.168.1.68, WSL Ubuntu)

- Mooncake binary at `~/.local/bin/mooncake` (cross-compiled
  linux/amd64 from this worktree, deployed via SCP).
- agentd is **running** as `aleh`, listening on `0.0.0.0:7878`,
  started via `nohup` after manual `pkill` during redeploy. PID was
  ~46309. **Dies on reboot** — no systemd unit yet (that's PR10).
  Restart command: `~/.local/bin/mooncake agentd --bind 0.0.0.0:7878 &`
- Token at `~/.config/mooncake/agentd.token`.
- Synced state at `~/.local/state/mooncake/agentd/synced/<controller>/<dirhash>/`
  — the entire `~/dotfiles` tree, cached so reruns HEAD-skip 100%.
- Run history at `~/.local/state/mooncake/agentd/runs/` — multiple
  runs from this session, including the final 31-changed success.
- Dotfiles are now applied: python3, rustup, go, java, neovim with
  packer plugins, tmux, zsh setup, ssh-server enabled, etc.

### On the Windows host (192.168.1.68)

- Two firewall rules named `WSL2 Mooncake Agentd` (one Windows
  Firewall, one Hyper-V) — already committed to dotfiles
  `platforms/windows/bootstrap.yml`. Applied manually this session.
- The Windows-side OpenSSH (`ssh aleh@192.168.1.68`) runs as
  Administrator — useful for these admin-only flows. Don't run
  mooncake against it directly; the WSL one (port 2222) is the
  "Linux peer" you provision.

---

## To resume work on another machine

### 1. Get the code

```bash
# If mooncake isn't cloned yet on the target:
git clone git@github.com:alehatsman/mooncake.git ~/projects/mooncake
cd ~/projects/mooncake

# Pick up the unpushed branch — you'll need to push it from x1 first:
#   x1$ cd ~/projects/mooncake/.claude/worktrees/epic-personal-fleet
#   x1$ git push -u origin worktree-epic-personal-fleet
# Then on the target:
git fetch origin
git checkout worktree-epic-personal-fleet
```

### 2. Build + smoke-test

```bash
cd ~/projects/mooncake
go build -o ~/mooncake ./cmd
go test ./... -count=1 -timeout 120s | tail -5    # 73 packages ok
```

### 3. Reconnect to the fleet

If you're on the PC and want to apply FROM the PC TO the x1 laptop:

```bash
# Start agentd on x1 first (over SSH from PC):
ssh x1 'nohup ~/.local/bin/mooncake agentd --bind 0.0.0.0:7878 >/tmp/agentd.log 2>&1 &'
# Read x1's token:
X1_TOKEN=$(ssh x1 'cat ~/.config/mooncake/agentd.token')
# Pair (replace <ip> with x1's LAN IP, ~/dotfiles agentd port unchanged):
~/mooncake fleet pair --name x1 --tag arch --token-via literal:"$X1_TOKEN" <x1-ip>:7878

# Apply your dotfiles to x1:
cd ~/dotfiles && ~/mooncake fleet apply x1
```

If you want to apply FROM the PC TO itself (loopback), the agentd is
already running on `localhost:7878`; just pair with that.

### 4. Continue the epic

The next planned PR is **PR6 — parallel multiplexer + ^C banner**.
Per `docs-working/specs/personal-fleet/implementation-order.md`:

> Replaces the serial loop in `cmd/fleet.go:fleetApplyAction` with
> parallel goroutines, adds padded `[host]` alignment, color, NO_COLOR
> respect, and the "remote runs continue" banner on ^C. ~400 LOC.

Files to touch:
- `cmd/fleet.go` — split out the per-peer loop, run as goroutines,
  feed a shared event channel.
- New `internal/fleet/multiplex.go` — the actual multiplexer (event
  channel reader, padding, color hash, writer goroutine).
- Wire ^C → cancel context → print "remote runs continue" banner →
  let goroutines drain.

Existing tests cover the building blocks. Add a new
`multiplex_test.go` with two fake peers and assertions on output
shape.

---

## Open questions / TODOs flagged this session

- **Long-line splitting on the daemon side** — packer-style `\r`
  rewritten progress lines should be split (at, say, `\r` or 64 KiB
  boundaries) before they become single huge `step.stdout` events.
  Currently band-aided with a 16 MiB SSE buffer.
- **`fleet apply` arg ordering** — urfave/cli requires positional
  arg last. `mooncake fleet apply main_pc --tag x` works;
  `mooncake fleet apply --tag x main_pc` fails. Fixable.
- **No `fleet status`** — to check peer reachability we still hand-
  curl `/v1/version`. PR7 fixes.
- **Bootstrap UX warts** — works, but: no systemd unit (so agentd
  dies on reboot), no rollback on partial failure, shell-outs to
  system `ssh`/`scp` instead of native `golang.org/x/crypto/ssh`.
  Spec-40 PR9-11 cover the proper version.

---

## Quick sanity commands

```bash
# Is the remote agentd still alive?
TOKEN=$(ssh -p 2222 aleh@192.168.1.68 'cat ~/.config/mooncake/agentd.token')
curl -sS -H "Authorization: Bearer $TOKEN" http://192.168.1.68:7878/v1/version

# Re-bootstrap from anywhere if the daemon died:
GOOS=linux GOARCH=amd64 go build -o /tmp/mooncake-linux-amd64 ./cmd
~/mooncake fleet bootstrap --port 2222 --name main_pc \
  --binary /tmp/mooncake-linux-amd64 aleh@192.168.1.68

# Apply dotfiles end-to-end:
cd ~/dotfiles && ~/mooncake fleet apply main_pc

# Run tests:
cd ~/projects/mooncake && go test ./... -count=1
```
