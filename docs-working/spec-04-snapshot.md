# Spec 04: Snapshot Command

## Problem

An AI agent starting work on an unknown machine needs to answer:
"what is installed here, what's running, how much space is there?"

Currently this requires 5-10 shell commands and burns 2k+ tokens parsing prose
output. mooncake already collects facts — but the output is verbose JSON with
fields an agent rarely needs.

## Goal

`mooncake snapshot` prints a compact, token-efficient picture of the machine
in one shot. Default output fits in ~400 tokens.

```
os: linux/arch x86_64  kernel: 6.14.4  host: thinkpad  user: alehatsman
hw: 16 cores (Intel i7-1365U)  ram: 14.2GB free / 32GB  disk: 45GB free / 512GB
uptime: 2d 4h

tools:
  git: 2.49.0   nvim: 0.11.0  tmux: 3.5a   zsh: 5.9
  go: 1.26.3    rust: 1.86.0  node: 22.14  python: 3.13.3
  docker: 27.4  fzf: 0.62.0   rg: 14.1.1

services (failed): thermald
services (stopped): ollama
```

## CLI interface

```bash
mooncake snapshot                    # default: compact text, ~400 tokens
mooncake snapshot --format json      # full structured JSON
mooncake snapshot --budget 200       # tighter token budget
mooncake snapshot --diff prev.json   # what changed since prev.json
```

## Token budget behavior

Fields are ranked by priority. When output would exceed `--budget` tokens,
lower-priority fields are dropped first:

1. os/arch/kernel/host/user  (always included)
2. hw summary (cores, ram free/total, disk free/total)
3. tool versions
4. services (failed first, then stopped; running services omitted unless --verbose)
5. uptime
6. CPU model, GPU, network interfaces  (dropped first)

Budget is estimated in tokens (1 token ≈ 4 chars). Not exact — best effort.

## Tool inventory (extends existing facts)

Current `Facts` already has: git, go, docker, python, ollama.

Add detection for:
- `nvim` / `vim`
- `tmux`
- `zsh` / `bash` / `fish`
- `node` / `npm`
- `rust` / `rustc` / `cargo`
- `java`
- `fzf`, `ripgrep` (`rg`), `fd`, `bat`, `eza`
- `kubectl`, `helm`, `terraform`
- `curl`, `wget`

Detection: `<tool> --version 2>&1` with a 2s timeout per tool. Cache with 5m
TTL (facts caching already exists). Failures → omit from output (not an error).

## Service state

Query systemd (Linux) or launchctl (macOS):
- Linux: `systemctl list-units --state=failed --no-legend` and
  `systemctl list-units --state=inactive --no-legend` (for key services only)
- macOS: `launchctl print-disabled` (omit for v1 if complex)
- Show only failed + key stopped services. Don't list all running services —
  too noisy, low signal.

## JSON format

`mooncake snapshot --format json` emits the full structured snapshot:

```json
{
  "ts": "2026-05-12T10:30:00Z",
  "os": {"name": "linux", "distro": "arch", "kernel": "6.14.4", "arch": "x86_64"},
  "host": {"name": "thinkpad", "user": "alehatsman", "uptime_s": 187200},
  "hw": {"cpu_model": "Intel i7-1365U", "cpu_cores": 16, "ram_total_mb": 32768, "ram_free_mb": 14540, "disk_total_gb": 512, "disk_free_gb": 45},
  "tools": {"git": "2.49.0", "nvim": "0.11.0", "go": "1.26.3", ...},
  "services": {"failed": ["thermald"], "stopped": ["ollama"]}
}
```

## Snapshot diff

`mooncake snapshot --diff prev.json` compares current snapshot against a saved
JSON file and prints only what changed:

```
tools:
  + rust: 1.86.0  (was: not installed)
  ~ go: 1.26.3    (was: 1.25.7)
services:
  + failed: thermald  (was: running)
```

Useful for: agent tracking state between sessions, drift detection, post-install
verification.

## Implementation

New `cmd/snapshot.go` command.

New `internal/snapshot/snapshot.go` — orchestrates facts collection + tool
inventory + service state, applies token budget, formats output.

Tool inventory: new `internal/facts/toolchains.go` additions (file already
exists, has `DockerVersion`, `GitVersion`, `GoVersion`). Extend the existing
`toolchains.go` with the full tool list above.

Service state: new `internal/facts/services.go` with platform-specific
implementations. Mirror the darwin.go/linux.go pattern already used.

## Out of scope

- Windows service state (v1 Linux/macOS only)
- Per-service details (just name + state)
- Network topology, open ports
- Installed package list from package manager
