# Spec 28: `os.*` System Config — cron, systemd, firewall, mount, sysctl

**Status:** Phases 1–6 complete + os.cron + os.sysctl reverse-
capture shipped. P1–P5 shipped earlier (os.systemd, os.cron,
os.sysctl, os.firewall ufw driver, os.mount). **Phase 6 (spec-22
ABI hooks) shipped**: Permissions / Diff / Cost / Reverse declared
on all five handlers using the testutil helpers. Reverse-capture
shipped for two handlers so far:

- **os.cron** (v2): captures `{Path, PriorExisted, PriorContent}`
  via the existing readFile call in computePlan; Reverse returns a
  cross-action `file.write` step (state=absent if the file didn't
  exist pre-apply, state=file with prior content otherwise).
- **os.sysctl** (v3): captures `{Name, AppliedState, PriorRuntimeValue,
  HadPriorRuntime, PriorPersistValue, HadPriorPersist, TouchedPersistFile,
  TouchedRuntime}`; Reverse builds an os.sysctl inverse step that
  either re-asserts the prior persist line + reload, or removes the
  added line (Persist=false), or restores the runtime-only mutation.
  Returns a defensive error when the apply mutated runtime but no
  prior runtime value was readable.

- **os.mount** (v4): captures `{Dest, PriorEntry, PriorMounted,
  TouchedFstab, TouchedMount}` (where `PriorEntry` snapshots the
  fstab line — Src, FSType, Options, Dump, Pass). Reverse builds an
  os.mount inverse step picking state=mounted / fstab_only / absent
  based on the captured (entry, mounted) tuple. Refuses the rare
  case "manually mounted with no fstab entry" since os.mount can't
  express it.

Reverse on the other two (os.systemd, os.firewall) still refuses
pending each handler's apply-time capture (own follow-up). Phase 7
(docs + non-ufw firewall drivers + macOS launchd if added later) is
the only outstanding work; non-ufw drivers remain deferred per the
spec's original scope.
**Epic:** E9 Modern Action Surface — bucket E9.3
**Effort:** L (2 weeks)
**Value:** Medium-high. Closes the "remaining OS config" gap. Less
agent-touched than identity/pkg/git, but the absence of these is
visible in every preset that does heavy systems work.

**Design principles:** `docs-working/action-design-principles.md`

---

## Problem

Five system-config domains, each with a clear "ensure this state"
pattern, none with a first-class action today:

1. **`os.cron`** — schedule jobs. Today: `shell: crontab` or manually
   write `/etc/cron.d/` files.
2. **`os.systemd`** — write + reload unit files. Today: `file.write`
   then `shell: systemctl daemon-reload`.
3. **`os.firewall`** — manage rules. Today: `shell: ufw`, `shell:
   nft`, or worse.
4. **`os.mount`** — `/etc/fstab` + actual mount. Today: `text.line`
   for fstab + `shell: mount`.
5. **`os.sysctl`** — kernel parameters. Today: `text.line` for
   `/etc/sysctl.d/` + `shell: sysctl -p`.

Each is small individually but together they cover the long tail of
host-config tasks.

---

## Goals

- **G1** `os.cron` — per-user crontab entries with named identity.
- **G2** `os.systemd` — write a unit file, reload daemon, enable, start.
- **G3** `os.firewall` — abstracted across ufw / firewalld / nftables /
  pf (Linux + macOS + BSD). Linux-first.
- **G4** `os.mount` — declare a mount + `/etc/fstab` entry.
- **G5** `os.sysctl` — kernel parameter management.
- **G6** All five implement spec-22 hooks.

**Out of scope:**

- `at` jobs / `systemd-timer` (timer files belong inside `os.systemd`).
- macOS launchd — separate `os.launchd_plist` action; Tier-2.
- Windows scheduled tasks — Tier-2.
- iptables raw rules (pre-nftables) — out of scope; users on legacy
  iptables can `shell` it.

---

## Design

### `os.cron`

```yaml
- os.cron:
    name: backup-nightly
    user: deploy
    state: present
    minute: 0
    hour: 3
    command: /opt/backup/run.sh
    env:
      MAILTO: ops@example.com
```

Implementation: write to `/etc/cron.d/<name>` (or `/var/spool/cron/
<user>` depending on platform). The `name` is the identity for
idempotency.

Output sugar:
```yaml
- os.cron:
    name: gc
    schedule: "*/15 * * * *"     # alternative to minute/hour/...
    command: ./gc.sh
```

### `os.systemd`

```yaml
- os.systemd:
    name: myapp.service
    state: present
    unit:
      Description: "My app"
      After: network.target
    service:
      ExecStart: /opt/myapp/bin/run
      Restart: on-failure
      User: deploy
    install:
      WantedBy: multi-user.target
    enabled: true
    started: true
    reload_on_change: true
```

`state: absent` removes the unit file, disables, stops.

Reload semantics: writing the unit file triggers `systemctl
daemon-reload`, which is captured as a separate "side effect" in the
plan (not a separate Step — internal to this action). Avoids the
common bug of "wrote unit but didn't reload daemon".

For timers, use `name: myapp.timer` and provide `timer:` instead of
`service:`.

### `os.firewall`

```yaml
- os.firewall:
    backend: ufw           # ufw | firewalld | nftables | pf
    state: present
    rule:
      protocol: tcp
      port: 22
      action: allow
      from: any            # or specific CIDR
      comment: ssh
```

Multi-rule:

```yaml
- os.firewall:
    backend: ufw
    rules:
      - { port: 22, protocol: tcp, action: allow, comment: ssh }
      - { port: 80, protocol: tcp, action: allow, comment: http }
      - { port: 443, protocol: tcp, action: allow, comment: https }
```

Idempotency: parse current rules via `ufw status numbered` / `nft list
ruleset`; compare; add/remove only deltas.

Backend selection: `backend: auto` (default) picks based on what's
installed and active.

### `os.mount`

```yaml
- os.mount:
    src: /dev/sdb1
    dest: /data
    fstype: ext4
    options: [defaults, noatime]
    state: mounted        # mounted | unmounted | fstab_only | absent
    backup: true          # snapshot /etc/fstab
```

Writes the fstab entry + mounts (or unmounts) as needed. `state:
fstab_only` writes the entry without mounting (useful for boot-time
mounts on volumes that aren't yet attached).

### `os.sysctl`

```yaml
- os.sysctl:
    name: net.ipv4.ip_forward
    value: 1
    state: present
    persist: true         # write to /etc/sysctl.d/99-mooncake.conf
    reload: true          # apply via sysctl -p
```

Idempotency: read current value via `sysctl -n`; compare; set if drift.

---

## Cross-cutting

All five declare `Sudo: true` and `RequiredBinaries: [...]`. Reverse
semantics:

| Action | Reversible | Mechanism |
|---|---|---|
| `os.cron` | ✓ | snapshot the cron file; restore |
| `os.systemd` | ✓ | snapshot unit file + daemon reload state |
| `os.firewall` | ✓ | snapshot rule set |
| `os.mount` | partial | unmount is fine; reformatted volumes can't be reversed |
| `os.sysctl` | ✓ | snapshot previous value |

---

## Key files

| File | Change |
|---|---|
| `internal/actions/os_cron/`, `os_systemd/`, `os_firewall/`, `os_mount/`, `os_sysctl/` | Five new handlers. |
| `internal/config/config.go` | Five new Step fields. |
| `internal/register/register.go` | Register five. |
| `internal/config/schema.json` etc. | Regenerate. |
| `internal/actions/os_firewall/ufw.go`, `nftables.go`, `firewalld.go` | Per-backend drivers behind a small interface. |

---

## Tasks (phased)

Each action is independent; can ship one at a time. Order by
priority:

1. **Phase 1** — `os.systemd` (highest demand). Service + timer
   coverage.
2. **Phase 2** — `os.cron`.
3. **Phase 3** — `os.sysctl`.
4. **Phase 4** — `os.firewall`. ufw first; nftables + firewalld
   follow.
5. **Phase 5** — `os.mount`.
6. **Phase 6** — spec-22 hooks across all five.

---

## Acceptance criteria

- `os.systemd` writes a unit, reloads daemon, enables, starts. Second
  run: idempotent.
- `os.cron` adds and removes a job; `crontab -u deploy -l` reflects
  the state.
- `os.firewall` with ufw adds 3 rules; second run: noop. Reverse
  removes them.
- `os.mount` mounts an ext4 image; reverse unmounts.
- `os.sysctl` sets `net.ipv4.ip_forward=1` and persists across
  reboots (verified via `/etc/sysctl.d/99-mooncake.conf`).
- All five implement spec-22 hooks.
- Build / vet / lint / test green.

---

## Open questions

1. **`os.firewall` zoned model (firewalld) vs flat (ufw)** — how to
   unify the YAML schema?  Probably expose both as alternative shapes
   per backend, with a common subset.
2. **`os.systemd` validation** — should we run `systemd-analyze
   verify` before reloading?  Yes; catches typos early.
3. **`os.mount` for tmpfs / overlay** — first-class or just
   pass-through via `fstype:`? Pass-through suffices.
4. **Timer files inside `os.systemd`** — same action, dispatch by
   filename suffix? Yes; if `name` ends in `.timer`, expect `timer:`
   block.
