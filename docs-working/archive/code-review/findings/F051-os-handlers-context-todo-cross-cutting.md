---
id: F051
title: Cross-cutting — os_* handlers call subprocess with context.TODO() (11 sites, 6 packages); mount/umount/ufw can hang indefinitely on NFS or driver wedge
severity: risk
package: cross-cutting
files:
  - internal/actions/os_user/platform_linux.go:79
  - internal/actions/os_user/platform_darwin.go:266
  - internal/actions/os_user/platform_darwin.go:281
  - internal/actions/os_group/platform_linux.go:62
  - internal/actions/os_group/platform_darwin.go:167
  - internal/actions/os_firewall/handler.go:468
  - internal/actions/os_firewall/handler.go:480
  - internal/actions/os_sysctl/handler.go:548
  - internal/actions/os_mount/handler.go:573
  - internal/actions/os_mount/handler.go:586
  - internal/actions/os_ssh_key/handler.go:592
status: open
verified: 2026-05-27 — `grep -rn 'context\.TODO()' internal/actions/os_*` returns 11 matches across 6 packages on master @ 4db53ad6. `actions.Context` (the per-step context passed to handlers, `internal/actions/interfaces.go:45`) does not expose a `context.Context` — handlers have no upstream cancel signal to plumb through. Same architectural gap as F016 (agentd.Worker), F014 (fleet.Apply WithoutCancel), F042 (facts.Collect, fixed) — `os_*` is the next family in the same class.
---

## What

Every `os_*` action handler shells out to a privileged tool
(`mount`, `umount`, `sysctl`, `ufw`, `usermod`, `groupadd`, `dscl`,
`chown`) via the per-package `runner.Run` helper:

```go
// internal/actions/os_mount/handler.go:573
out, err := runner.Run(context.TODO(), "mount", dest)
```

`context.TODO()` is a placeholder context with no deadline, no
cancellation channel, and no parent — equivalent to
`context.Background()` for `exec.CommandContext`. The subprocess
will run to completion or until the kernel kills it; the handler
cannot interrupt it.

11 call sites across 6 packages:

| File | Line | Command | Hang trigger |
|---|---|---|---|
| `os_mount/handler.go` | 573 | `mount <dest>` | **stuck NFS mount, broken automount target** |
| `os_mount/handler.go` | 586 | `umount <dest>` | **busy mount with open file handles, NFS** |
| `os_firewall/handler.go` | 468 | `ufw <args>` | netfilter/iptables lock contention, ufw daemon wedge |
| `os_firewall/handler.go` | 480 | `ufw status numbered` | same |
| `os_sysctl/handler.go` | 548 | `sysctl -w name=value` | typically fast, but blocks on some hardware sysctls |
| `os_user/platform_linux.go` | 79 | `useradd / usermod / userdel` | rare; nss/sssd lookups can hang under network identity providers |
| `os_user/platform_darwin.go` | 266 | `dscl` | same on macOS Directory Service |
| `os_user/platform_darwin.go` | 281 | `useradd / usermod` shim | same |
| `os_group/platform_linux.go` | 62 | `groupadd / groupmod / groupdel` | rare; same nss path |
| `os_group/platform_darwin.go` | 167 | `dscl` | same |
| `os_ssh_key/handler.go` | 592 | `chown <spec> <path>` | typically fast |

The two **realistic, observed-in-prod** hang triggers are:

1. **Stuck NFS mount** — `mount <nfs-target>` blocks indefinitely
   when the server is unreachable and the share is configured with
   default (`hard,intr`) options. `umount` on a busy NFS share has
   the same hang shape.
2. **netfilter lock contention** — `ufw` (and the underlying
   `iptables`/`nft`) takes an exclusive lock; if another process
   holds it (a misbehaving container engine, an in-progress
   `firewalld` restart), the call waits forever.

When the operator presses Ctrl-C during a hung `mount`/`ufw`, the
SIGINT path (`cmd/kernel/apply.go:runWithSignalCtx`) cancels the
parent context — but the handler's subprocess is started with
`context.TODO()`, so it ignores the cancellation. The
`os.Exit(130)` in `runWithSignalCtx:251` then kills the daemon
without draining the child; the child gets reparented to init and
keeps running (mount mounts the share, ufw mutates the firewall)
**after** the apply has reported "aborted".

## Same family

- **F012** — cross-cutting HTTP-without-timeout (9 packages, fixed)
- **F014** — `fleet.Apply` `WithoutCancel` masks Ctrl-C (fixed)
- **F016** — `agentd.Worker` uses `context.Background` (fixed)
- **F027** — agentd self-upgrade `sanityCheckBinary` no timeout (fixed)
- **F042** — `facts.Collect` 29 exec.Command without ctx (fixed via
  `internal/facts/exec.go` probe helpers wrapping
  `exec.CommandContext` with a 5s `probeTimeout`)

`os_*` is the same shape as F042: many short-lived subprocess
calls, no ctx parameter on the seam, fix is to add per-call
timeouts at the lowest helper.

## Why it isn't already fixed

`actions.Context` (`internal/actions/interfaces.go:45`) exposes
`GetTemplate`, `GetEvaluator`, `GetLogger`, `GetVariables`,
`GetEventPublisher` — but no `Context() context.Context`. Handlers
have no upstream cancel signal to thread through. Two options for
the fix:

1. **Lowest-impact** — mirror F042's pattern: each `os_*` package's
   `runner.Run` wrapper (already an indirection seam for testing)
   gets a per-call timeout. Default 30s for mount/umount (long
   enough for slow but healthy NFS), 5s for everything else. No
   change to `actions.Context`. Operators with deliberately slow
   targets override via a new `timeout` field on the action.
2. **Architectural** — add `GetContext() context.Context` to
   `actions.Context`, plumb the apply-level signal-cancel context
   through the executor → handler. Bigger blast radius (touches
   every handler, including the ~30 already-clean ones), but lets
   handlers honor Ctrl-C and `--timeout` uniformly.

Recommendation: option 1 for this finding (matches F042's
precedent and resolves the user-visible hang); option 2 belongs
under the F016 follow-up that's already tracked.

## Repro

```yaml
- name: hang on stuck NFS mount
  os.mount:
    src: 192.0.2.1:/share     # RFC 5737 unrouteable
    dest: /mnt/dead
    fstype: nfs
    opts: hard,intr
    state: mounted
```

Running this on a host without the share already in `/etc/fstab`
will block forever inside `runner.Run(context.TODO(), "mount", "/mnt/dead")`.
Ctrl-C prints the abort banner; `ps -ef | grep mount` shows the
`mount.nfs` child still running after the daemon exits.

## Fix sketch

Add a `runCmd(timeout)` helper in each package (or shared in
`internal/actions/exec/`) that wraps `exec.CommandContext` with a
`context.WithTimeout`:

```go
func runCmd(timeout time.Duration, name string, args ...string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
```

Per-package timeouts:
- `os_mount` mount/umount: 30s default (override via field)
- `os_firewall` ufw: 10s
- `os_sysctl`: 5s
- `os_user`/`os_group` userdb mutations: 10s (sssd/nscd slow paths)
- `os_ssh_key` chown: 5s

The lowest-impact change is package-local; the cross-cutting helper
is a refactor that can land later. The behaviour change (timeout
where there was none) is opt-out via the new `timeout` field, so
existing healthy plans are unaffected.
