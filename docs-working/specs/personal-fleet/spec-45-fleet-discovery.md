# Spec 41: Fleet Discovery — mDNS + SSH Config + Static Peers

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md), sub-epic P3.
**Status:** Draft
**Effort:** M (~1 week)
**Value:** High — turns "how do I add my four boxes" from a manual TOML-
editing chore into `mooncake fleet init` plus answering some prompts.
**Depends on:** spec-43 (peers.toml format, peer transport). Bootstrap UX
(spec-47) builds on this.

---

## Problem

After spec-43, peers are managed by hand-edited `~/.config/mooncake/peers.toml`.
Fine for one peer. Painful for four. Worse, the user has to know each peer's
IP/hostname, bearer token, and tags before they can start.

We want three discovery sources, merged into a single candidate list:

1. **Static `peers.toml`** — already the source of truth. Listed for
   completeness.
2. **mDNS / DNS-SD** — agentds advertise themselves on the LAN; the
   controller queries for them. Zero-config on home WiFi.
3. **SSH config import** — `~/.ssh/config` already has named hosts the user
   trusts. Surface them as bootstrap candidates.

These compose under one command: `mooncake fleet init`.

---

## Goals

- **G1** Agentd advertises `_mooncake._tcp.local` on its bind port with a
  TXT record containing `version` and `hostname` (no token, no sensitive
  data).
- **G2** Controller can query that service to list candidate peers on the
  LAN.
- **G3** Controller can parse `~/.ssh/config` and surface named hosts as
  bootstrap candidates (no agentd assumed there).
- **G4** `mooncake fleet init` merges all three sources into a candidate
  list, prompts to add each, and updates `peers.toml`.
- **G5** Authentication and discovery are kept strictly separate. mDNS tells
  you a peer *exists*; it does NOT tell you the bearer token, and it does
  NOT establish trust. Pairing (token exchange) is a separate step.

**Non-goals:**

- Authenticated mDNS (e.g. signed TXT records). Discovery is informational.
- Internet-scale discovery (UPnP, public registries, etc.).
- Continuous discovery (the controller listening forever). Discovery is a
  one-shot query at `init` time.
- Tailscale / Consul / etcd integration. Reconsider if asked.

---

## Reuse map

**Reused:**

- `peers.toml` format + loader from spec-43.
- agentd's existing TCP listener and `GET /v1/version` from spec-43 (used
  to validate that a discovered peer is actually mooncake).

**Extended:**

- `internal/agentd/server.go` — start an mDNS advertise goroutine when
  `cfg.BindAddr != ""`.

**New:**

| Component | Location |
|---|---|
| mDNS responder/query lib wrapper | `internal/fleet/discovery/mdns.go` |
| SSH-config parser | `internal/fleet/discovery/sshconfig.go` |
| Discovery aggregator | `internal/fleet/discovery/aggregate.go` |
| `fleet init` CLI command | `cmd/fleet.go` (extends spec-43 scaffold) |

---

## mDNS service shape

**Service type:** `_mooncake._tcp.local`

**TXT record fields** (per Bonjour conventions, key=value pairs):

```
v=1                   # protocol version
hn=arch-laptop        # peer's hostname (operator-configurable, see below)
ver=0.9.0             # mooncake version
sm=user               # mode: "user" or "system"
```

Explicitly NOT included:
- Bearer token (or any prefix thereof).
- IP address — handled by the SRV/A records already.
- Any user data.

### Configurable advertise name

The peer's mDNS instance name defaults to OS hostname (with `.local`
stripped). But OS hostnames are messy on personal fleets ("MacBook-Air.local"
vs "macbook"). agentd accepts a `--name <id>` flag and a `[advertise].name`
field in `agentd.toml` (new file, see open questions) to override.

If the operator sets a name, that's what shows in TXT `hn=`. Otherwise:
strip `.local`, lowercase, replace whitespace with `-`.

### Library choice

Use `github.com/grandcat/zeroconf` (pure-Go, no native deps, works on Linux
and macOS). Confirmed unmaintained-but-stable; fallback to
`hashicorp/mdns` if zeroconf proves flaky in practice.

Avoid shelling out to `avahi-publish` or `dns-sd`: portability and
deployment fragility.

---

## SSH config import

Parse `~/.ssh/config` for `Host <name>` entries:

- Skip hosts with wildcards in their name (`*.example.com`).
- Skip `Host *` block.
- Extract `HostName`, `User`, `Port`.
- For each host, surface as a candidate `(name, addr, transport=ssh-bootstrap)`.
  The init flow treats these as **bootstrap candidates** rather than
  agentd-ready peers — selecting one runs spec-44 bootstrap.

Use `github.com/kevinburke/ssh_config` or write a small parser inline.
ssh_config is ~300 LOC and handles `Match` blocks; we only need `Host`
+ the three fields, so inline is fine.

---

## Aggregation logic

`AggregateCandidates(opts) ([]Candidate, error)`:

1. Load current `peers.toml`. Build a set of already-configured peer names
   and addresses.
2. Run mDNS query, 3-second window. For each response, build a candidate
   with `source = "mdns"`.
3. Parse `~/.ssh/config`. For each host, build a candidate with
   `source = "ssh-config"`.
4. Deduplicate. Order of preference: existing peers.toml > mdns >
   ssh-config (an SSH-config entry that matches an mDNS hit is collapsed
   into the mDNS one; the SSH config can still feed the bootstrap path if
   selected).
5. Return a single list of `Candidate` structs.

```go
type Candidate struct {
    Name      string   // operator-visible name
    Addr      string   // host:port for agentd, host for ssh-only
    Source    string   // "mdns" | "ssh-config" | "peers.toml"
    AgentdUp  bool     // true if mDNS found, or peers.toml entry pings ok
    SSHReady  bool     // true if ssh-config saw it
    Tags      []string
}
```

---

## CLI: `mooncake fleet init`

Interactive flow. The pretty path:

```
$ mooncake fleet init
discovering candidates…

  source        name           addr                    status
  ────────────  ─────────────  ──────────────────────  ─────────────
  peers.toml    laptop         laptop.lan:7878         ✓ agentd up
  mdns          desktop1       desktop1.local:7878     ✓ agentd up (mooncake 0.9.0)
  mdns          macbook        macbook.local:7878      ✓ agentd up (mooncake 0.9.0)
  ssh-config    vps-1          vps-1.example.com:22    ssh-only, not bootstrapped

Add new peers to ~/.config/mooncake/peers.toml? [Y/n] Y

? desktop1 — name in peers.toml (default desktop1):  ↵
? desktop1 — tags (comma-separated, optional): linux,workstation
? desktop1 — paste bearer token (cat /etc/mooncake/agentd.token on desktop1):
  > xxxxx
   ✓ verified (mooncake 0.9.0)

? macbook — name: ↵
? macbook — tags: darwin,workstation
? macbook — paste bearer token: xxxxx
   ✓ verified

? vps-1 — bootstrap over SSH now? [y/N] N
   skipped; run `mooncake fleet bootstrap vps-1` later.

wrote ~/.config/mooncake/peers.toml (3 peers, 1 skipped)
✓ fleet ready: `mooncake fleet status` to verify.
```

### Non-interactive mode

`--accept-all` skips prompts and:
- For each mDNS candidate: prints a clear ERROR with "no token source"; the
  user MUST run with `--from-bootstrap` or paste tokens. We don't auto-trust.
- Recommend: combine `fleet init --accept-all` with `fleet bootstrap` for
  SSH-config candidates.

### Token pairing UX

The hard step is moving the peer's bearer token to the controller. Three
paths offered in v1:

1. **Manual paste**: operator runs `cat <token_path>` on the peer, pastes
   into the prompt. Default.
2. **SSH pull**: `--ssh-fetch user@host` SSHes in, reads the token file,
   uses it. Optional.
3. **Bootstrap**: `mooncake fleet bootstrap` (spec-44) handles end-to-end.

For init flow v1, only path 1 is implemented; paths 2 and 3 are deferred to
spec-47 (bootstrap UX). Initial workflow assumes the user can `cat` the
file on each peer.

---

## Wire shape for `GET /v1/version` (extend)

To make verification meaningful, the daemon's version response gains a
`hostname` and `synced_root` field (the latter required by spec-43 anyway):

```json
{
  "version":      "0.9.0",
  "hostname":     "desktop1",          // matches the advertised hn= if set
  "synced_root":  "/var/lib/mooncake/agentd/synced",
  "system_mode":  true,
  ...
}
```

The init flow uses this to confirm "yes, the box at this address is the one
mDNS thinks it is" before writing the peer entry.

---

## Tasks

### Task 1 — mDNS advertise (daemon-side)

1. New `internal/fleet/discovery/mdns.go`:
   - `Advertise(ctx, opts AdvertiseOptions) error` blocks until ctx is done.
   - Uses `grandcat/zeroconf`.
2. Wire into `internal/agentd/server.go:Serve`:
   - Start a goroutine when `cfg.BindAddr != ""` and `cfg.AdvertiseMDNS` is
     true (default true; `--no-mdns` to disable).
   - Pass `cfg.AdvertiseName` (from `--name` flag or `hostname` fallback).
3. `cmd/agentd.go`: add `--name`, `--no-mdns` flags.

### Task 2 — mDNS query (controller-side)

1. `Query(ctx, timeout) ([]Candidate, error)` in the same package.
   Runs a Bonjour query for `_mooncake._tcp` with the supplied timeout.
2. For each response, do a `GET /v1/version` (unauth, no token needed) to
   sanity-check it's a live mooncake. Failures downgrade to "candidate
   only, agentd unreachable".

### Task 3 — SSH config parser

1. `internal/fleet/discovery/sshconfig.go`:
   - `Parse(path string) ([]Candidate, error)`.
   - Inline parser, handles `Host`, `HostName`, `User`, `Port`. Ignores
     `Match`, `Include`, wildcards.
2. Standard config locations: `~/.ssh/config`, then system
   `/etc/ssh/ssh_config` if `--include-system` passed.

### Task 4 — Aggregator

1. `internal/fleet/discovery/aggregate.go`:
   - `AggregateCandidates(opts) ([]Candidate, error)` runs the three
     sources in parallel goroutines (each with timeout), merges results.

### Task 5 — `fleet init` command

1. Extend `cmd/fleet.go` with `init` subcommand.
2. Interactive prompts (use `github.com/AlecAivazis/survey` or a small
   custom prompt — pick the existing dependency if mooncake already uses
   one elsewhere; otherwise inline).
3. Write peers.toml in a `peers.toml.tmp` + rename for atomicity.

### Task 6 — Daemon-side `/v1/version` extension

1. Extend the version response struct (existing handler in
   `internal/agentd/handlers.go:versionHandler`) with `hostname` and
   `synced_root` fields. Both XS additions.

### Task 7 — Tests

1. mDNS advertise/query round-trip on loopback (skip on platforms where
   loopback mDNS doesn't work — document in test skip).
2. SSH config parser: handle each of the known config quirks.
3. Aggregator: dedup correctness when same host appears in multiple
   sources.
4. `fleet init --accept-all --non-interactive` smoke test against two
   mocked candidates (no real network).

---

## Open questions

1. **Is mDNS worth the dep cost?** zeroconf is ~5k LOC. For a personal-fleet
   user with a static peer list, it's optional. Could ship init with
   SSH-config + manual-add only, defer mDNS to a follow-up. Lean: ship
   mDNS since the "discovery on home WiFi" demo is the wow moment.
2. **mDNS over WireGuard / Tailscale.** Multicast typically doesn't work
   over these overlays. Document that mDNS is LAN-only; on Tailscale, use
   MagicDNS hostnames in `peers.toml` directly.
3. **Stable instance name on macOS.** macOS aggressively renames Bonjour
   instances when collisions occur ("desktop (2)"). The `--name` flag
   sidesteps this; ensure the daemon doesn't fight back when zeroconf
   suggests a different name.
4. **Agentd token file location in system mode.** Default `/etc/mooncake/agentd.token`
   is root-only. The "paste from cat" UX in init requires the operator to
   SSH or otherwise read it. Could expose `GET /v1/auth/token` over the
   unix socket only (never TCP) so a local user can fetch it. Defer.
5. **`agentd.toml` config file.** This spec gestures at `[advertise].name`
   but agentd is currently flag-only. Either keep `--name` only or add a
   TOML config file. Lean: flags suffice for v1; add config file when we
   have 3+ flags worth keeping.
