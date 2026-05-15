# Bug — `fleet upgrade` linux fails with EXDEV across staging vs install mounts

**Surfaced:** 2026-05-15 during the spec-56 / spec-57 redeploy session.
**Repro frequency:** every linux peer where `/var/lib` and `/usr/local`
live on different filesystems. On x1's WSL Ubuntu (`main_pc`):

```
[main_pc] ✗ replace: peer main_pc: POST /v1/self/replace: HTTP 500:
  swap_binary (rename /var/lib/mooncake/agentd/upgrade/staged-37c9c37d
  → /usr/local/bin/mooncake: rename
  /var/lib/mooncake/agentd/upgrade/staged-37c9c37d
  /usr/local/bin/mooncake: invalid cross-device link)
```

WSL maps `/var/lib`, `/var/log`, etc. via overlayfs or onto the
distro-image layer; `/usr/local` sits on a different physical mount.
That arrangement is common enough that `fleet upgrade` should expect
it.

---

## Root cause

`internal/agentd/self_upgrade_*.go` (linux + darwin paths) finalises
the binary swap with `os.Rename(stagedPath, finalPath)`. `rename(2)`
returns `EXDEV` when the source and destination filesystems differ,
and Go surfaces that as `*os.PathError{Op:"rename", Err:syscall.EXDEV}`
with the message we see above.

The path is **on purpose** — `os.Rename` is the only atomic way to
swap a file at a fixed name on POSIX. But it's only atomic when both
paths are on the same filesystem; the upstream design assumed that
holds for `/var/lib/mooncake/agentd/upgrade/` ↔ `/usr/local/bin/`.

Three failure modes downstream:

1. **Replace fails (current bug).** The daemon stays on the old
   binary; the controller reports a clean "upgrade failed" but with
   an opaque kernel-level error.
2. **Staged file leaks.** The pre-rename staging file at
   `/var/lib/mooncake/agentd/upgrade/staged-*` is left on disk
   indefinitely. Re-running `fleet upgrade` adds another one;
   nothing GCs them.
3. **Manual recovery is the only escape.** The controller's
   `fleet upgrade` output suggests rerun, but the rerun keeps
   hitting EXDEV. Operator has to SSH in and `cp + mv` themselves.

---

## Fix

Fall back to copy-then-remove on `EXDEV`. Atomicity is preserved
*against concurrent readers* via the standard write-tmp-then-rename
within the destination directory:

```go
err := os.Rename(staged, final)
if err == nil { return nil }
var pErr *os.PathError
if !(errors.As(err, &pErr) && errors.Is(pErr.Err, syscall.EXDEV)) {
    return err
}
// Cross-device. Copy into a sibling tmpfile of the *destination*,
// then rename within the destination's filesystem.
tmp := final + ".swap." + randomSuffix()
if err := copyFile(staged, tmp, 0o755); err != nil { return err }
if err := os.Rename(tmp, final); err != nil {
    _ = os.Remove(tmp)
    return err
}
_ = os.Remove(staged)  // best-effort GC of the source
return nil
```

`copyFile` should:
- Open `tmp` with `O_WRONLY|O_CREATE|O_TRUNC|O_CLOEXEC`
- Set the mode bits explicitly (don't rely on umask)
- `fsync` before close so the rename hits a complete file

Tests: a `withTempfsBoundary(t)` helper that mounts a tmpfs at the
"staging" path inside `t.TempDir()` would let `swap_binary_test.go`
exercise both the same-fs and cross-fs branches.

---

## Workaround in the field

Until the fix lands, operators recover by SSHing to the peer and
finishing the swap by hand:

```sh
ssh peer
sudo cp /var/lib/mooncake/agentd/upgrade/staged-<sha8> \
        /usr/local/bin/mooncake.new
sudo mv /usr/local/bin/mooncake.new /usr/local/bin/mooncake
sudo systemctl restart mooncake-agentd
```

`fleet upgrade --binary <path>` should be re-run after to confirm.

---

## Scope of platform impact

| Platform | Path used                                            | EXDEV risk                                              |
|----------|------------------------------------------------------|---------------------------------------------------------|
| linux    | `/var/lib/mooncake/agentd/upgrade/` → `/usr/local/bin/` | **High.** WSL, containers, multi-partition setups.      |
| darwin   | `/var/db/mooncake/agentd/upgrade/` → `/usr/local/bin/`  | Low. `/var` and `/usr/local` are both `/` on stock macOS, but Time Machine / data-volume layouts can vary. |
| windows  | `%LOCALAPPDATA%\Mooncake\agentd\upgrade\` → `%LOCALAPPDATA%\Mooncake\bin\` | None. Same drive by design. |

Windows already does the right thing (paths are siblings under
`%LOCALAPPDATA%\Mooncake\`); the live fleet upgrade against
`main_pc-win` worked clean during the same session.

---

## Related bug

`fleet upgrade`'s `await restart` step has a too-short timeout for
S4U-principal Windows tasks. During the same session, the first
Windows upgrade reported `context deadline exceeded` even though the
binary swap succeeded — PID 684 was still serving on the old
in-memory binary because the scheduler hadn't restarted the process
yet. A `Stop-ScheduledTask` + `Start-ScheduledTask` round-trip then
brought it onto the new binary cleanly. Probably a separate spec /
bug; tracking here as a "while you're in the area" note.
