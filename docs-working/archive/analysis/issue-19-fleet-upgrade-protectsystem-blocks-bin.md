# Bug — `fleet upgrade` linux still fails after #12 fix: `ProtectSystem` blocks writes to `/usr/local/bin`

**Tracking:** [#19](https://github.com/alehatsman/mooncake/issues/19)
**Surfaced:** 2026-05-15 during the retest after issue #12's EXDEV
fallback (`9797b89f`) landed.

## Repro

```
$ mooncake fleet upgrade main_pc --binary /tmp/mooncake-master
[main_pc] replace…
[main_pc] ✗ replace: peer main_pc: POST /v1/self/replace: HTTP 500:
  swap_binary (cross-fs swap: copy
  /var/lib/mooncake/agentd/upgrade/staged-bf7199b5 →
  /usr/local/bin/mooncake.upgrade-tmp: open
  /usr/local/bin/mooncake.upgrade-tmp: read-only file system)
```

The EXDEV fallback from #12 is exercised correctly. The new failure
mode is `EROFS` (read-only filesystem) when the agentd tries to
create `/usr/local/bin/mooncake.upgrade-tmp`.

## Root cause

The shipped systemd unit
(`internal/fleet/init/mooncake-agentd.service`) has
`ProtectSystem=true`:

```ini
NoNewPrivileges=true
ProtectSystem=true
ProtectHome=false
```

Per `systemd.exec(5)`, `ProtectSystem=true` mounts `/usr`, `/boot`,
and `/efi` read-only in the daemon's mount namespace. The
fleet-upgrade code path needs to write to `/usr/local/bin/` to swap
the binary atomically — and that path is under `/usr`, so the
write fails with `EROFS`.

The same barrier was there before #12 — the EXDEV fallback simply
made it more visible because the new error message reveals which
syscall failed. Pre-#12 the rename failed with EXDEV first on
cross-fs setups (and presumably with EROFS on same-fs setups,
though no operator had reported it).

This is also why same-FS macOS / non-WSL linux installs might have
"worked": when ProtectSystem is silently disabled (or the user
installed the unit by hand without ProtectSystem), `os.Rename`
between same-fs paths succeeded. Once the embedded unit's
hardening reached production, the breakage became latent and only
visible via `fleet upgrade`.

Coupled with #12, the bug surfaces today as: **linux `fleet
upgrade` was never able to complete via the canonical path on any
host where the embedded unit is the one that's actually running.**

## Fix

Whitelist `/usr/local/bin` for write access in the unit:

```ini
NoNewPrivileges=true
ProtectSystem=true
ProtectHome=false
ReadWritePaths=/usr/local/bin
```

`ReadWritePaths` punches a hole in the `ProtectSystem` read-only
overlay for exactly the directory the daemon needs. It's the
canonical pattern for "harden everything except this one path"
under systemd.

Also reasonable but invasive: change the install target to
`/opt/mooncake/bin/mooncake` (under `/opt` which is writable by
default). That moves the discovery / PATH story — operators expect
`mooncake` on PATH, and `/usr/local/bin` is the right default for
that — so this isn't free.

## Workaround

Same as before: SSH in and finish the swap manually.

```sh
ssh peer
STAGED=$(sudo bash -c "ls -t /var/lib/mooncake/agentd/upgrade/staged-* | head -1")
sudo cp "$STAGED" /usr/local/bin/mooncake.new
sudo mv /usr/local/bin/mooncake.new /usr/local/bin/mooncake
sudo systemctl restart mooncake-agentd
```

The `sudo mv` works because `cp` ran as the operator (not the
daemon), and the operator's shell isn't under the daemon's
sandbox. The daemon itself still can't do this swap.

## Test gap

A `fleet upgrade` integration test that runs against a real (or
test-double) systemd-managed agentd would catch this. The current
test for the EXDEV fallback (`self_upgrade_swap_test.go`) exercises
the Go-level `os.Rename` + copy logic in isolation, with no
sandbox active.

Also worth adding: a smoke test in CI that boots a tiny linux VM
or container, installs the embedded unit, runs `fleet upgrade`
against it, and asserts the swap actually completes.

## Related — same-fs setups have the latent EROFS too

Even on a system where `/var/lib` and `/usr/local` live on the same
filesystem (so #12's EXDEV path is never taken), the original
`os.Rename(staged, /usr/local/bin/mooncake)` would have failed
with `EROFS` due to ProtectSystem. The reason fleet upgrade had
been reported "working" historically is probably one of:

1. Hand-installed unit without `ProtectSystem=*`
2. The fix for #8 (this session) moved from `ProtectSystem=full`
   to `ProtectSystem=true` — both make /usr RO, so neither helps,
   but `=full` would also have failed and we don't have a previous
   "fleet upgrade succeeded" data point to compare against.

Recommend pairing the `ReadWritePaths` fix with a quick survey of
which peers in the fleet have which version of the unit installed
(via `fleet exec --peer-filter os=linux 'systemctl cat
mooncake-agentd | head -20'`).
