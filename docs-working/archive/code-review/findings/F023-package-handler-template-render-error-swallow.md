---
id: F023
title: package handler silently swallows template-render errors on package names → unrendered {{var}} reaches the package manager
severity: bug
package: internal/actions/package
file: internal/actions/package/handler.go
lines: 142-149
status: done
fixed: 2026-05-16 — commit a855ec13 `fix(package): F023 — return render errors on pkg names instead of silently keeping the literal`. Extracted a `renderPackageNames(ctx, names) ([]string, error)` helper (handler.go:199-209) that returns the first render error wrapped as `render package name %q: %w`. Both the legacy `Execute()` swallow site and the spec-16 `Run()` site were patched. Subsequent commit 4fffeb3e deleted Execute/DryRun entirely (spec-16 close-out), so `Run` is the only path today and calls `renderPackageNames` at line 725 before any branch.
verified: 2026-05-17 — confirmed fixed on master @ b6b79712. (a) Tests green: `TestRun_RenderErrorOnPackageName_Apply`, `_Plan`, and `TestRun_RenderErrorOnPackageName` all pass (`go test -count=1 -run TestRun_RenderError ./internal/actions/package/...`). The apply + plan tests prove both modes share the renderPackageNames call. (b) Manual repro with `pkg.name: "{{ unclosed-tools"`: caught by config-validation BEFORE the handler runs (`failed to compile step ... Line 1 Col 13 near 'tools' '}}' expected`) — a broader-than-F023 fix in the validator. (c) Two observations worth noting for whoever touches this next: (i) F023's original reproducer text claims "If the variable is unset, the template engine returns an error" — this is wrong about pongo2-in-this-codebase, which renders undefined vars to empty string. The test comments at run_test.go:71-75 already call this out: the realistic trigger is a syntax / filter error, not a missing var. The finding's "Why it's a bug" section overstates the surface. (ii) TestRun_RenderErrorOnPackageName (run_test.go:112-129) has a stale comment claiming Execute is still wired in — Execute was deleted in 4fffeb3e. The test is now a duplicate of `_Apply` and could be removed in passing.
---

## What

`Handler.Execute` builds the package list, then renders each name
through the template engine. On render error, **the loop keeps
the original (unrendered) name** and continues:

```go
packages := h.buildPackageList(pkg)
for i, name := range packages {
    rendered, renderErr := ctx.GetTemplate().Render(name, ctx.GetVariables())
    if renderErr == nil {
        packages[i] = rendered
    }
    // ← no else branch; render errors are silently dropped
}
```

Two ways this surfaces, both bad:

**1. Apparent install of a literal placeholder.** If
`pkg.Name = "{{ os_pkg }}-tools"` and the variable is unset, the
template engine returns an error. `packages[0]` stays
`"{{ os_pkg }}-tools"`. The next call invokes:

```sh
apt-get install -y "{{ os_pkg }}-tools"
```

apt refuses (invalid package name) and the step fails with a
**confusing error from apt** — not from mooncake's template
engine — making the misconfiguration much harder to debug:

```
E: Unable to locate package {{ os_pkg }}-tools
```

A user staring at that has no signal that the issue was a missing
variable on the mooncake side.

**2. Partial expansion is even worse.** If a names list has 10
entries and entry 3 fails to render, entries 1-2 + 4-10 expand
correctly and entry 3 is sent as `{{ ... }}`. apt may install
the 9 that resolved AND fail on the literal — leaving the system
in a partial state with no clean rollback (PkgReverseInfo at
line 428-432 only records what's in `toInstall`, which now
includes a literal name).

## Why it's a bug, not a smell

Reproducible:

```yaml
- pkg:
    name: "{{ undefined_variable }}"
```

Today: step fails with `E: Unable to locate package {{
undefined_variable }}`.

Expected: step fails with a clear "template render failed:
undefined variable 'undefined_variable'" message before any
package-manager call.

## Suggested fix

Return the render error to the caller:

```go
for i, name := range packages {
    rendered, renderErr := ctx.GetTemplate().Render(name, ctx.GetVariables())
    if renderErr != nil {
        return nil, fmt.Errorf("render package name %q: %w", name, renderErr)
    }
    packages[i] = rendered
}
```

That's the consistent shape with `resolveNamesExpr` (line 154)
right below it, which DOES return the render error:

```go
if pkg.NamesExpr != "" {
    expanded, expandErr := h.resolveNamesExpr(ctx, pkg.NamesExpr)
    if expandErr != nil {
        return nil, fmt.Errorf("failed to resolve package names expression %q: %w", pkg.NamesExpr, expandErr)
    }
    packages = append(packages, expanded...)
}
```

Two helpers wrong about the same thing in different ways: line
143-149 silently keeps the broken value; line 154-158 returns the
error. The latter is correct.

## Adjacent observation

`Handler.DryRun` (line 188-237) skips the rendering entirely:

```go
packages := h.buildPackageList(pkg)
if pkg.NamesExpr != "" {
    if expanded, expandErr := h.resolveNamesExpr(ctx, pkg.NamesExpr); expandErr == nil {
        packages = append(packages, expanded...)
    } else {
        ctx.GetLogger().Infof("  Would expand names from expression: %s", pkg.NamesExpr)
    }
}
```

So `mooncake plan` for a pkg step with `{{ var }}-tools` shows
the unrendered literal in dry-run output but would fail apply.
Plan-mode prediction is misleading.

The Spec-16 `Run()` path likely shares this — verify when fixing.
Either:

- Plan mode renders the names through the same path Execute uses,
  so the plan output shows what would actually run.
- Plan mode explicitly notes "names contain unresolved templates;
  apply will fail" when rendering fails at predict time.

## Verification

- Add test `TestPackageHandler_UnresolvedTemplateName`: a pkg
  step with `Name: "{{ undefined }}"` should error out of Execute
  before any exec.Command, with a "render package name" message.
- `go test ./internal/actions/package/...`
- Manual: `mooncake apply` with a pkg step using an undefined
  variable — error should now name the variable.

## References

- `internal/actions/package/handler.go:154` — the correctly-shaped
  cousin (resolveNamesExpr).
- F008 (tool's renderToolTemplates) — adjacent pattern but
  *doesn't* have this bug; that function returns the error
  consistently across all 9 fields.
