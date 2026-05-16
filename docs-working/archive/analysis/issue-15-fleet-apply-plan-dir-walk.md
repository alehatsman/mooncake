# Bug — `fleet apply` fails to walk plan-dir containing non-regular files

**Tracking:** [#15](https://github.com/alehatsman/mooncake/issues/15)
**Surfaced:** 2026-05-15 during the spec-56 / spec-57 retest cycle, while
exercising `fleet apply` with a minimal heartbeat plan.

**Repro:**

```sh
$ cat > /tmp/heartbeat.yml <<'EOF'
- name: Heartbeat
  log:
    msg: "hi from {{ hostname }}"
EOF
$ mooncake fleet apply /tmp/heartbeat.yml
fleet apply: /tmp/heartbeat.yml → 2 peer(s)
[main_pc-win] ✗ walk plan-dir: non-regular file in plan-dir:
    /tmp/.X11-unix/X0 (mode=Srw-rw-rw-)
[main_pc    ] ✗ walk plan-dir: non-regular file in plan-dir:
    /tmp/.X11-unix/X0 (mode=Srw-rw-rw-)
fleet apply: 0/2 ok
```

The X11 abstract-domain socket has filesystem mode bits matching a
*socket* (`S` prefix) not a regular file. `fleet apply` syncs the
whole plan-dir to each peer and refuses to proceed when it
encounters a non-regular file. `/tmp` on any Linux box has a few of
those at any time (X11 socket, sometimes a wayland-0 socket,
sometimes systemd-private-* dirs).

---

## Root cause

`fleet apply`'s default plan-dir is the directory containing the
plan file (per `--plan-dir`'s help text: "default: directory of
the plan file"). The internal sync walker (`internal/fleet/sync/`
or similar — needs a code-path trace) treats any non-regular file
as a hard error.

The contract makes sense in spirit — fleet sync wants to push a
self-contained tree of templates, presets, imports — but it's too
strict in practice:

- Sockets, FIFOs, device files are never plan content.
- Symlinks (often present in nested layouts) deserve a sane policy
  too: follow them, copy as symlinks, or refuse with a clear
  message.
- The user invoking `mooncake fleet apply /tmp/x.yml` is in 99% of
  cases *not* asking to sync all of `/tmp`; they just chose a
  convenient throw-away file location.

---

## Fix — three layers

### 1. Skip non-regular files silently

In the walker (where the current "non-regular file" error
originates), add a per-mode filter:

```go
if !info.Mode().IsRegular() {
    // Skip sockets, FIFOs, devices, etc. — never plan content.
    // Symlinks get a separate decision (see option 2 below).
    return nil
}
```

This unblocks the common case (heartbeat plan in /tmp).

### 2. Add a smarter default for unspecified --plan-dir

When the user runs `fleet apply /path/to/single-file.yml` without
`--plan-dir`, and the directory containing the file is a well-known
"shared scratch" location (`/tmp`, `/var/tmp`, `~/Downloads`,
`os.UserCacheDir()`), default to syncing *only the single plan
file* rather than the whole directory. Detect via:

- The file is the only `.yml` / `.yaml` in the directory, OR
- The plan has no `import:` / `template src:` / `use:` references
  (a simple grep is fine; this is a heuristic, not a parser)

Pair with a warning so the user knows what got synced:

```
[main_pc] sync: 1 uploaded, 0 skipped (auto-isolated heartbeat.yml;
          pass --plan-dir to share more files)
```

### 3. Refuse cleanly when --plan-dir is /tmp or $HOME

If the user explicitly passes `--plan-dir /tmp` or another
filesystem root, error out with a clear message rather than walking:

```
fleet apply: --plan-dir cannot be a shared scratch directory
  (/tmp). Use a dedicated subdirectory.
```

---

## Workaround in the field

Put the plan file in its own directory:

```sh
mkdir -p /tmp/my-plan && mv /tmp/heartbeat.yml /tmp/my-plan/
mooncake fleet apply /tmp/my-plan/heartbeat.yml
```

This works because `/tmp/my-plan/` contains only the plan file (no
sockets / FIFOs to trip the walker).

---

## Related observation: walker error mode

The current error includes the full mode string (`Srw-rw-rw-`)
which is helpful for diagnosis. Keep that in the eventual
soft-skip path's debug log so an operator chasing a missing
template can still see what got skipped:

```
[main_pc] sync: 1 uploaded, 0 skipped (skipped 1 non-regular file:
          /tmp/.X11-unix/X0 mode=Srw-rw-rw-)
```
