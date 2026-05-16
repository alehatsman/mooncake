# Stream: fleet — Manual Test Plan

Tests for `agentd`, the peer transport, and the `fleet` subcommands.
Anything that turns single-box apply into multi-machine orchestration.

> **Prereq**: the core test plan must pass first. fleet stacks on top
> of `mooncake apply` — broken kernel = broken fleet.

## What to test

| Surface | What "correct" looks like |
|---|---|
| **agentd lifecycle** | Starts, listens on TCP + Unix socket, emits structured request logs, shuts down cleanly on SIGTERM |
| **Bearer-token auth** | Unauthenticated requests get 401; valid token gets 200; file at `~/.config/mooncake/agentd.token` is auto-generated |
| **Peer discovery** | `fleet status` reports per-peer accessibility; mDNS / SSH-config / `peers.toml` all feed into the inventory |
| **fleet exec** | Streams events with `[peer]` line prefixes; stdout from remote appears verbatim; final summary shows `1/1 ok` or `1/2 ok — unreachable: X` |
| **fleet observe** | Cross-peer typed observation returns a comparison table; one peer down doesn't break others' rows |
| **fleet doctor** | Probe ladder: resolve → tcp → http → auth → facts, with green/red per rung |
| **Multi-peer fan-out** | `--peer-filter tag=X` selects subsets; `name=X` exact-matches; empty match exits cleanly with "nothing to do" |
| **Per-host overlays** | `vars/by-host/<peer>.yml` loads when targeting that peer; `--overlays off` opts out |
| **Run audit** | Each fleet run lands a record in agentd's run history with ULID; `fleet logs <peer>` streams events from the latest |

## Test environment recipe

```bash
# Build once
CGO_ENABLED=0 go build -ldflags='-s -w' -o out/mooncake-static ./cmd

# Spin up agentd + a single self-peer in one container
docker run --rm \
  -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro \
  ubuntu:24.04 bash -c '
    mooncake agentd --bind 127.0.0.1:7878 --no-mdns >/dev/null 2>&1 &
    sleep 1
    TOK=$(cat /root/.config/mooncake/agentd.token)
    cat > /root/.config/mooncake/peers.toml <<EOF
[[peers]]
name = "local"
addr = "127.0.0.1:7878"
token = "$TOK"
tags = ["test"]
EOF
    mooncake fleet status
    mooncake fleet exec "echo hi"
  '
```

**Why `--no-mdns`**: docker containers can't reliably advertise mDNS.
Skip the discovery layer when you're testing fleet primitives.

**Why two containers for "real" fleet tests**: a single-container
self-peer can't catch listener-binding issues or peer-isolation bugs.
For those, spin up agentd in container A and run `mooncake fleet` in
container B, with both on the same docker network.

## Critical TOML gotcha

`peers.toml` is **array-of-tables**, NOT dotted-table:

```toml
# CORRECT
[[peers]]
name = "local"
addr = "127.0.0.1:7878"
token = "..."
tags = ["test", "dev"]

# WRONG — produces: "toml: cannot store a table in a slice"
[peers.local]
addr = "127.0.0.1:7878"
```

This bites everyone the first time (see #78). Always copy from a
known-good example.

## Test scenarios

### 1. Single-peer happy path (5 min)

```bash
# In one container
mooncake agentd --bind 127.0.0.1:7878 --no-mdns &
sleep 1
# Configure peer as above
mooncake fleet status           # → 1/1 accessible
mooncake fleet exec "uname -s"  # → Linux
mooncake fleet facts local      # → JSON facts
mooncake fleet observe cpu      # → comparison table
mooncake fleet doctor local     # → probe ladder, "→ healthy"
mooncake fleet ps               # → "no in-flight runs"
mooncake fleet logs local       # → latest run's events
```

All five subcommands should succeed.

### 2. Unreachable peer (5 min)

Add a fake peer to `peers.toml`:
```toml
[[peers]]
name = "fake"
addr = "127.0.0.1:9999"
token = "bogus"
tags = ["test"]
```

```bash
mooncake fleet status           # → 1/2 accessible, "fake" unreachable
mooncake fleet exec "echo hi" --peer-filter tag=test
# Expected: [local] succeeds, [fake] fails with diagnostic hint
# "port open but no listener — agentd not running?"
```

### 3. Tag- and name-filtering

```bash
mooncake fleet exec "..." --peer-filter tag=test       # both peers
mooncake fleet exec "..." --peer-filter name=local     # one peer
mooncake fleet exec "..." --peer-filter tag=nonexistent
# Expected: "fleet exec: --peer-filter selected 0 of 2 peer(s); nothing to do"
```

### 4. Bootstrap + pair error paths

```bash
mooncake fleet bootstrap fake-user@127.0.0.1
# Expected: "ssh connect: no auth methods available
#   (start ssh-agent or place ~/.ssh/id_ed25519)"

mooncake fleet pair --help
# Verify --token-via stdin|file:|literal: is documented
```

### 5. Two-peer cross-host (advanced, requires docker compose)

For real fleet behavior, spin up two containers on the same docker
network, each running agentd. Configure host A's peers.toml to point
at host B's IP. Run `fleet exec` and `fleet observe` and confirm:
- Stdout from B appears prefixed `[hostB]` on host A's terminal
- Run on B is visible in B's `mooncake history` (the controller doesn't
  own the run history)
- `fleet ps` on A shows the run in-flight on B

## Tricks & tips

1. **agentd's token is at `~/.config/mooncake/agentd.token`**, not
   in the state-dir. 43 chars, base64-ish. Cat it directly to wire
   into peers.toml.

2. **Use `mooncake fleet doctor <peer>` for every flake.** The probe
   ladder names exactly which rung fails — TLS / TCP / HTTP / auth /
   facts. Beats `curl -v` and reading mooncake logs.

3. **Run IDs are ULIDs**: 26-char, sortable, globally unique. Grep
   them in agentd logs to trace a specific run end-to-end.

4. **`[peer]` line prefix is consistent** across `fleet exec`, `fleet
   logs`, `fleet apply`. If you're parsing fleet output, anchor on
   `^\[<peer>\] `.

5. **The agentd HTTP API is documented in itself.** Run agentd with
   `--bind 127.0.0.1:7878`, then `curl -H "Authorization: Bearer
   <token>" http://localhost:7878/v1/` returns route inventory.
   Test against this directly — it's faster than `fleet` round-trips.

6. **agentd HTTP request logs include `request_id`.** When debugging a
   confusing fleet error, grep the agentd log for the run's ULID;
   you'll find every HTTP roundtrip with timing.

7. **Hostname masking matters.** `fleet exec` line prefixes use a
   *display name* that replaces dots/slashes with dashes
   (`127-0-0-1` for `127.0.0.1`). When grepping logs across hosts,
   normalize first.

8. **mDNS is flaky in docker.** Use `--no-mdns` on agentd to avoid
   spurious "renamed" warnings; pair with explicit `peers.toml`.

## Common pitfalls

- **Don't forget `--token` matching `~/.config/mooncake/agentd.token`**
  on the agentd's host. Wrong token = 401, peer shown as
  "unreachable" with confusing auth-failed message.

- **TCP vs Unix socket**: agentd binds both by default. `fleet`
  always uses TCP (because cross-host). The Unix socket is for
  local IPC (faster). Test both paths if you change the API.

- **Concurrent runs on one agentd**: not yet stress-tested. Don't
  assume the queue is bounded; if you submit 100 fleet exec in
  parallel, look at agentd RSS.

- **agentd doesn't survive SIGINT mid-run cleanly** (#87 — same
  class as `mooncake apply`). When testing interrupt handling, look
  for orphan child processes and incomplete run audit entries.

## How to file findings

Same convention as core. The fleet-specific bin is:
`docs-working/analysis/findings-<DATE>/positive-keepers.md` (fleet
features that work — they're the project's biggest demo) and
`cli-and-friction.md` (peers.toml format errors, missing hints).

## Concrete priority targets

If you have one hour:

1. **Single-peer happy path** (5 subcommands × verify) — regression
   test for the basic fleet contract
2. **Two-peer with one down** — regression test for partial failure
   surfacing
3. **`fleet exec` with stdout-heavy command** (`yes | head -100`) —
   regression test for stream multiplexer flow control
4. **`fleet upgrade`** (push new agentd binary) — un-tested in 2026-05-15
   audit; high priority gap
5. **agentd `/v1/files` PUT under `--max-sync-bytes` limit** —
   never tested, contract unclear
