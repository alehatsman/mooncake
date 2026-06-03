---
id: fleet
status: draft
owners: [aleh]
covers:
  - cmd/fleet/**
  - internal/fleet/**
---

# Fleet — multi-host peer-to-peer orchestration

## Intent

The fleet layer lets one operator drive mooncake plans across many machines they
own from a single controller, with no central hub: every box runs an equal
`agentd` peer, the controller holds only a `peers.toml` roster, and each `fleet`
subcommand fans out to the selected peers in parallel over HTTP+SSE (agentd) or
SSH (diagnostics/bootstrap only). It owns discovery, transport, remote exec,
daemon bootstrap/install, and fleet-wide apply/status.

## Behavior

- WHEN `fleet <cmd>` runs without an explicit selection it targets every peer in
  `peers.toml`; WHEN `--peer` is given it selects by name, `key=value` filter, or
  `@k=v,...` AND-group, UNIONing repeated `--peer` flags.
- WHERE a peer's `transport` is `agentd`, the controller reaches it over
  `http://addr/v1/*` with a bearer token; WHERE it is `ssh`, that channel is used
  only for bootstrap and `fleet doctor` fallback — never to apply runs.
- WHEN `fleet discover` runs it aggregates candidates from `peers.toml`,
  `~/.ssh/config`, and mDNS, dedups by name, sorts, and optionally probes each
  agentd peer's `/v1/version` — without mutating any state.
- WHEN `fleet apply <plan|machine>` runs it syncs the plan tree to each selected
  peer, submits a run, and multiplexes per-peer event streams into one
  `[host] …` log; IF any peer fails the command reports a non-zero outcome.
- WHEN `fleet status` runs it probes each peer's `/v1/version`, `/v1/runs`, and
  `/v1/facts` in parallel and renders STATE (ok/running/failed/unreachable), OS,
  version, queue depth, and last-run outcome; `--json` emits JSONL.
- WHEN a peer is unreachable, the transport classifies the network error into a
  human cause label (DNS / refused / timeout) rather than surfacing a raw stack.
- WHEN `fleet doctor <peer>` runs it walks a probe ladder (DNS → TCP → HTTP),
  optionally adding an SSH-fallback rung when `peer.ssh` is configured.
- WHEN `fleet bootstrap user@host` runs it executes the 8-step SSH sequence
  (connect → detect platform → idempotent same-version short-circuit → upload
  binary → render+install service unit → start+wait reachable → read token →
  upsert `peers.toml`); IF the local binary's format/arch does not match the
  detected target it fails fast before upload (#117); WHERE the Windows target's
  active network profile is Public it warns that the firewall rule won't apply
  (#118).
- WHEN `fleet up <peer>` runs and the peer is powered off it sends a Wake-on-LAN
  magic packet to the peer's stored MAC; WHEN `fleet shutdown` powers a peer off
  it first refreshes that MAC into `peers.toml`.
- WHEN `fleet exec`/`observe` run they synthesize a one-step shell/observation
  plan, distribute it, and capture each peer's outcome in parallel.
- WHEN `fleet ps`/`logs`/`watch` run they list in-flight runs / stream history /
  multiplex live events from agentd peers, skipping non-agentd transports with a
  warning.
- IF `fleet bootstrap` is interrupted, a rerun SHOULD resume from the first
  incomplete step rather than redo completed ones, and `--dry-run` SHOULD print
  the plan without mutating the target (#29).
- WHEN `fleet doctor --all` runs it SHOULD probe every peer in one pass, and
  `fleet status` SHOULD show each peer's last-applied plan + timestamp (#28).
- WHEN an operator needs interactive triage or a one-off file push they SHOULD
  use `fleet shell <peer>` / `fleet cp` over the audited agentd channel instead
  of dropping to raw SSH (#26).
- WHEN `fleet ps` shows an in-flight run the operator SHOULD be able to stop it
  with `fleet kill <peer> <run-id>` (#25).

## Non-goals

- No central controller, scheduler, or message bus — the controller is a thin
  fan-out client and peers never talk to each other.
- The SSH transport is not an apply path; it is strictly bootstrap + diagnostics.
- No cross-peer consensus, leader election, or shared cluster state.

## Checklist

- [x] `peers.toml` roster: parse/validate/upsert, atomic writes, MAC normalize (`internal/fleet/peers.go`)
- [x] Peer selection: name / `key=value` / `@`-group filters, UNION (`cmd/fleet/peer_flag.go`)
- [x] Discovery: peers.toml + ssh_config + mDNS aggregate, dedup, probe (`internal/fleet/discovery/**`)
- [x] agentd HTTP+SSE transport client + network-error classification (`internal/fleet/transport/**`)
- [x] Fleet apply: sync tree, submit run, multiplexed per-peer logs (`internal/fleet/{apply,orchestrator,multiplex,sync}.go`)
- [x] `fleet status` parallel health probe + `--json` (`cmd/fleet/status.go`)
- [x] `fleet doctor` probe ladder + SSH fallback rung (`cmd/fleet/doctor.go`)
- [x] 8-step SSH bootstrap/install across linux/darwin/windows (`internal/fleet/{bootstrap.go,install/**}`)
- [x] Bootstrap binary format/arch match check before upload (`internal/fleet/install/binverify.go`, #117)
- [x] Windows bootstrap Public-profile firewall warning (`internal/fleet/bootstrap_windows_target.go`, #118)
- [x] Wake-on-LAN `fleet up` + MAC auto-refresh on shutdown (`internal/fleet/wol/**`, `cmd/fleet/{up,shutdown,mac_refresh}.go`)
- [x] `fleet exec` / `observe` synthesized shell/observe plans (`internal/fleet/{exec,observe}/**`)
- [x] `fleet ps` / `logs` / `watch` run listing + live event multiplex (`cmd/fleet/{ps,logs,watch}.go`)
- [ ] Idempotent bootstrap with `--resume` + `--dry-run` (#29)
- [ ] `fleet doctor --all` + `fleet status` last-applied column (#28)
- [ ] Hoist `--no-color`/`--json`/`--peers-file`/`--parallel`/`--timeout` to global fleet flags (#27)
- [ ] `fleet shell <peer>` + `fleet cp` over the audited agentd channel (#26)
- [ ] `fleet kill <peer> <run-id>` to cancel an in-flight run (#25)
- [ ] macOS agentd preset coverage parity with Linux (#36)
