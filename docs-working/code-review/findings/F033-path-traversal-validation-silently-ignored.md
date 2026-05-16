---
id: F033
title: Cross-cutting — 11 call sites of pathutil.ValidateNoPathTraversal silently log + continue; repo_apply_patchset has a real escape, others are dead-code theater
severity: bug
package: cross-cutting
files:
  - internal/actions/repo_apply_patchset/handler.go:397
  - internal/actions/text_patch_ini/handler.go:110
  - internal/actions/text_patch_json/handler.go:121
  - internal/actions/text_patch_yaml/handler.go:112
  - internal/actions/text_line/handler.go:116
  - internal/actions/file_insert/handler.go:149, 431
  - internal/actions/file_patch_apply/handler.go:139, 563
  - internal/actions/file_delete_range/handler.go:139, 393
  - internal/actions/file_replace/handler.go:140, 389
status: open
verified: 2026-05-16 — confirmed real: repo_apply_patchset/handler.go:397 and text_patch_ini/handler.go:110 both call ValidateNoPathTraversal then log-and-continue (debug-level). Path-traversal validation is dead-code theater. 11 call sites, 9 packages affected
---

## What

`pathutil.ValidateNoPathTraversal` is called from 11 spots across
9 packages. The function returns an error if a path is absolute,
starts with `..`, or contains `..` as a component
(`pathutil/safety.go:58`).

`grep` of the call sites shows the same shape in 11 of 13 places:

```go
if pathErr := pathutil.ValidateNoPathTraversal(targetPath); pathErr != nil {
    ctx.GetLogger().Debugf("  Path validation warning for %s: %v", targetPath, pathErr)
}
```

**The error is logged at Debug level and the handler proceeds.**
Debug-level logs are suppressed at the default `info` log level, so
operators never see the warning.

The remaining 2 call sites (`unarchive/handler.go`,
`unarchive/idempotent.go`) correctly return the error.

## Two distinct issues

### (A) repo_apply_patchset — real path traversal escape

`repo_apply_patchset/handler.go:394-399`:

```go
targetPath := filepath.Join(baseDir, filePatch.Path)
if pathErr := pathutil.ValidateNoPathTraversal(targetPath); pathErr != nil {
    ctx.GetLogger().Debugf("  Path validation warning for %s: %v", targetPath, pathErr)
}
// Read original file
originalContent, err := os.ReadFile(targetPath)
// ... write patched content back to targetPath ...
```

Patchset payload supplies `filePatch.Path` (the `--- a/...` line of
a unified diff). A malicious patchset can contain
`--- a/../../etc/passwd`, which `filepath.Join` resolves through
`Clean` to a path outside baseDir. ValidateNoPathTraversal flags
it; the warning is silently logged; the handler reads and
overwrites the file at the escaped path.

`baseDir` is operator-supplied so the absolute risk depends on
trust: a patchset from a trusted registry/source can do this;
a patchset from an untrusted source absolutely can.

**Realistic exploit (when patchset is fetched from a remote
registry):**

```
diff --git a/../../etc/passwd b/../../etc/passwd
--- a/../../etc/passwd
+++ b/../../etc/passwd
@@ -1 +1 @@
-root:x:0:0:root:/root:/bin/bash
+attacker:x:0:0::/root:/bin/bash
```

Apply this patchset against `baseDir=/srv/app/v1` → `targetPath
= /srv/app/v1/../../etc/passwd` = `/etc/passwd`. The file is read,
patched, written back. Root account compromised.

Even for a "trusted" baseDir, the implicit invariant ("patchset
operates within baseDir") is violated.

### (B) Per-file handlers — dead-code theater

For the 10 other handlers (text_line, text_patch_{ini,json,yaml},
file_insert, file_replace, file_delete_range, file_patch_apply),
the `path` is `ec.Svc.PathUtil.ExpandPath(rawPath, currentDir,
vars)` — which **always produces an absolute path** for the
common config-supplied paths users write.

ValidateNoPathTraversal then ALWAYS returns "absolute path not
allowed" for these handlers. The Debug log fires on every step
that runs at default config (every `text.line` step, every
`text.patch.json` step, etc.).

This means:

1. The validation is **theater** — it can never succeed against
   the actual code path.
2. The "warning" is dead noise at debug level — operators tuning
   to debug see a flood of false positives.
3. The handler proceeds to do exactly what the user asked
   (read/write the absolute path). That's the **intended**
   behavior; users WANT to write to `/etc/myapp.yml`.

The wrong validator is being applied. ValidateNoPathTraversal is
for "joined paths that should stay within a base directory" (the
unarchive case). For absolute-path handlers, no path-traversal
check is meaningful — the user explicitly provided the path.

## Why it's a bug (not a smell)

(A) is exploit-able under realistic conditions. (B) is wrong code
that misleads readers into thinking validation is happening when
it isn't.

## Suggested fix

### (A) repo_apply_patchset: use SafeJoin or return the error

`pathutil/safety.go:85` already has `SafeJoin` that uses
`filepath.Rel` to verify the joined path stays under base:

```go
// Replace:
targetPath := filepath.Join(baseDir, filePatch.Path)
if pathErr := pathutil.ValidateNoPathTraversal(targetPath); pathErr != nil {
    ctx.GetLogger().Debugf(...)
}

// With:
targetPath, joinErr := pathutil.SafeJoin(baseDir, filePatch.Path)
if joinErr != nil {
    patchResult := &PatchResult{
        File:    filePatch.Path,
        Success: false,
        Error:   fmt.Sprintf("path escapes patchset base directory: %v", joinErr),
    }
    results = append(results, patchResult)
    if raps.Strict {
        h.rollbackChanges(backups)
        return results, false, fmt.Errorf("patch %s escapes base in strict mode: %w", filePatch.Path, joinErr)
    }
    continue
}
```

This refuses any `filePatch.Path` that escapes `baseDir`,
matching the implicit contract.

### (B) Per-file handlers: delete the dead check

For text_line, text_patch_{ini,json,yaml}, file_insert,
file_replace, file_delete_range, file_patch_apply: **delete the
ValidateNoPathTraversal call entirely**. The path is
user-supplied and absolute by design; "no traversal" doesn't
apply.

If a defense-in-depth check is desired (e.g. refuse paths under
`/proc`, `/sys`, `/dev`), add a different validator
(`ValidateNotSystemPath` or similar). But that's a NEW feature,
not a bug fix; not in scope for F033.

The simpler shape:

```go
// Just delete these lines from all 10 sites:
if pathErr := pathutil.ValidateNoPathTraversal(path); pathErr != nil {
    ctx.GetLogger().Debugf("text.patch.json: path validation warning: %v", pathErr)
}
```

Removes ~30 lines of dead theater. Same effective behavior.

## Adjacent — log-level mismatch elsewhere

The pattern "validate, log Debug, continue" appears here. Audit:
`grep -rn 'Debugf.*validation\|Debugf.*warning'` to find other
cases where security/correctness validation is suppressed to
debug. Quick audit suggests this pattern is unique to the
pathutil callers; other validators (`step.Validate()`, schema
validation) return errors normally.

## Verification

### For (A)

- Add `TestRepoApplyPatchset_RejectsTraversingPaths` with a
  patchset containing `--- a/../../etc/passwd` and assert the
  step fails. Pre-fix: writes /etc/passwd; post-fix: rejects.

### For (B)

- `grep -rn 'ValidateNoPathTraversal' internal/actions/` — only
  `unarchive` remains.
- `grep -rn 'Debugf.*validation warning' internal/actions/` —
  no hits.
- `go test ./internal/actions/...` — existing tests pass.

## References

- `pathutil/safety.go:85` — `SafeJoin` is the correct primitive
  for "joined path stays in base".
- `pathutil/safety.go:58` — `ValidateNoPathTraversal` is for raw
  pre-Join input that should not contain `..`. The current
  call-sites pass post-Join / pre-rendered paths to it, which is
  the wrong granularity.
- Unarchive handler — example of the correct shape (return the
  error, refuse to extract).
