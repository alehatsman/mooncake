# Bug — `file.write` symlink with `force: true` fails plan inspection when path is a non-symlink

**Status**: Open
**Discovered**: 2026-05-14, in [dotfiles](https://github.com/alehatsman/dotfiles) bootstrap (was `common.yml`, now `shared/bootstrap.yml`).
**Affects**: `mooncake plan` only — `mooncake apply` works correctly.

---

## Symptom

`mooncake plan` aborts during plan inspection on this task:

```yaml
- name: Link mooncake presets into per-user search path
  file.write:
    path: ~/.mooncake/presets
    src: ~/.mooncake-src/presets
    state: link
    force: true
```

When `~/.mooncake/presets` is a real directory (not a symlink), plan exits with:

```
failed to inspect plan: /home/<user>/.mooncake/presets exists and is not a symlink
```

No tags can work around it: `--tags` filtering happens *after* inspection, so even slices that don't reference this task hit the error.

`mooncake apply` on the same task succeeds — it correctly removes the existing path and creates the symlink. Only plan-mode prediction is broken.

## Reproduction

```sh
mkdir -p /tmp/mc-bug/presets-realdir
mkdir -p /tmp/mc-bug-src/presets
cat > /tmp/mc-bug/repro.yml <<'EOF'
---
- name: Force-link onto existing dir
  file.write:
    path: /tmp/mc-bug/presets-realdir
    src: /tmp/mc-bug-src/presets
    state: link
    force: true
EOF

mooncake plan -c /tmp/mc-bug/repro.yml
# → failed to inspect plan: /tmp/mc-bug/presets-realdir exists and is not a symlink
```

## Root cause

`internal/effects/default.go:207-229` — `defaultPerformer.Symlink(target, path, opts)`:

```go
case err == nil:
    e.Reason = "path exists and is not a symlink"
    e.Err = fmt.Errorf("%s exists and is not a symlink", path)
    return e
```

The non-symlink case returns a hard error unconditionally. The `Force` intent never reaches this code: `actions.PerformerOpts` (`internal/actions/performer.go:62`) carries only `Become` and `BecomeUser` — no `Force`. The handler's `checkLinkForce` (`internal/actions/file/handler.go:980`) catches the "symlink with different target" case but not the "path is not a symlink at all" case, and even if it did, `defaultPerformer.Symlink` would re-trip on its own check.

In Apply mode the same function recovers fine — line 236 `os.Lstat` + `os.Remove` removes whatever's there before `os.Symlink`. Plan-mode prediction is the only path that errors.

## Expected behaviour

With `force: true`, plan should report `WouldChange=true` with a reason like:

```
↑ Link mooncake presets into per-user search path   would replace directory with symlink -> /home/<user>/.mooncake-src/presets
```

No error, no aborted inspection.

## Fix sketch

1. **Thread `Force` through `PerformerOpts`** (or as a fourth arg to `Symlink`/`Hardlink`):

   ```go
   // internal/actions/performer.go
   type PerformerOpts struct {
       Become     bool
       BecomeUser string
       Force      bool
   }
   ```

2. **Handle non-symlink + `Force=true` in `defaultPerformer.Symlink`** (`internal/effects/default.go:220-223`):

   ```go
   case err == nil:
       kind := describeKind(info) // "directory" | "file" | "device" | …
       if opts.Force {
           e.Reason = fmt.Sprintf("would replace %s with symlink -> %s", kind, target)
           if p.modeFn() == actions.ModePlan {
               e.WouldChange = true
               return e
           }
           // Apply mode: existing removal at line 236+ already handles this.
       } else {
           e.Reason = "path exists and is not a symlink"
           e.Err = fmt.Errorf("%s exists and is not a symlink (use force: true to replace)", path)
           return e
       }
   ```

3. **Pass `Force` from `file.write` handler** when `state: link` and `force: true`. Same for hardlink.

## Test cases

Add to `internal/effects/default_test.go` (or wherever `Symlink` is covered):

- Path is a regular directory, `Force=false` → returns Err with helpful message.
- Path is a regular directory, `Force=true`, plan mode → `WouldChange=true`, no Err, reason mentions "directory".
- Path is a regular file, `Force=true`, plan mode → `WouldChange=true`, reason mentions "file".
- Path is a symlink pointing elsewhere, `Force=true`, plan mode → existing behaviour preserved (`WouldChange=true`, "symlink target differs").
- Path doesn't exist → existing behaviour preserved (`WouldChange=true`, "would create symlink").

End-to-end:
- Re-run the dotfiles bootstrap repro above; `mooncake plan` should succeed with the would-change line.

## Workarounds (current)

- Run `mooncake apply` instead of plan (works, but defeats the point of plan).
- Manually convert the existing dir to a symlink before planning.
