---
id: F024
title: plan.walkAndRender doesn't render templates inside map[string]interface{} fields — os.systemd / text.patch.json / use.with templates silently pass through
severity: bug
package: internal/plan
file: internal/plan/planner.go
lines: 787-869
status: fixed
---

## ✅ Fixed

`walkAndRender`'s `case reflect.Map` now accepts maps with element
kind `Interface` (not just `String`). For each entry it unwraps the
`reflect.Interface` to its concrete value; if the concrete is a
string, render and write back; otherwise pass through unchanged.
Non-string concretes (numbers, bools, nested maps, lists) leak
through templates today (the recursive case the finding flagged)
but the common case — string-valued entries in
`os.systemd.{Unit,Service,Timer,Socket,Install}`,
`text.patch.json.{Set,Merge}`, `text.patch.yaml.{Set,Merge}`, and
`use.With` — is now rendered correctly.

### Regression tests

`internal/plan/planner_f024_test.go`:

- `TestPlanner_RendersOsSystemdServiceSection` — a step with
  `os.systemd.service.ExecStart = "{{ binary_path }}"` and a
  defined `binary_path` var. Asserts the planned Service map has
  the rendered absolute path. Pre-fix the literal `{{ binary_path }}`
  passed through (verified by stashing the fix and re-running:
  `Service[ExecStart] = "{{ binary_path }}"` instead of the
  rendered value).
- `TestPlanner_OsSystemdNonStringValuesPreserved` — mixed-type
  Service map (string ExecStart, int TimeoutStopSec, string
  Restart). Asserts non-string entries pass through unchanged
  without reflect panics.

### Not in scope

- **Nested `map[string]interface{}` → `map[string]interface{}`**.
  Mentioned in the finding's note about the strict-correct
  recursive walk. Not common in practice for the action structs
  listed above (systemd unit sections are flat key→value). Worth
  a separate finding if a user hits it.
- **Defense-in-depth verification pass** (the finding's
  "errors on any string containing `{{` after rendering" suggestion)
  is broader than F024 and not bundled here — would catch every
  future closed-kind-set miss, so worth tracking separately.

---

## What

The planner's `walkAndRender` (`planner.go:787`) handles a closed
set of field kinds when rendering templates inside an action struct:

| Kind | Renders? |
|---|---|
| `reflect.String` | yes |
| `reflect.Pointer → String` | yes |
| `reflect.Pointer → Struct` | yes (deep-copy + recurse) |
| `reflect.Pointer → Map` | **no** |
| `reflect.Slice of String` | yes |
| `reflect.Slice of Struct` | **no** |
| `reflect.Map of String → String` | yes |
| `reflect.Map of String → interface{}` | **no** |

This is the exact same closed-kind-set issue F019 just fixed in
the secrets resolver. **Same root cause, different package, same
class of silent miss.**

## Action structs with unhandled fields

`grep -nE 'map\[string\]interface' internal/config/config.go` finds
at least 5 action structs where the closed kind set silently
passes templates through:

| Action | Field | Effect when templated |
|---|---|---|
| `os.systemd` | `Unit`, `Service`, `Timer`, `Socket`, `Install` — `map[string]interface{}` | `ExecStart: "{{ binary_path }}"` ends up in `/etc/systemd/system/foo.service` literally as `{{ binary_path }}`. systemd then refuses to load the unit. |
| `text.patch.json` | `Set`, `Merge` — `map[string]interface{}` | Templated upsert values stay as `{{ var }}` strings written to the target JSON. |
| `text.patch.yaml` | `Set`, `Merge` — `map[string]interface{}` | Same as JSON. |
| `use` (preset invocation) | `With map[string]interface{}` | Templated preset parameters reach the preset unrendered. The preset then renders again (since presets recurse through the planner), so this **partially works** by accident — but any preset that doesn't re-render breaks. |

## Concrete repro

```yaml
- vars:
    binary_path: /usr/local/bin/myapp

- os.systemd:
    name: myapp.service
    service:
      ExecStart: "{{ binary_path }}"
      Restart: on-failure
```

Expected: the generated unit's `ExecStart=` is
`/usr/local/bin/myapp`.

Actual: the planner doesn't render `{{ binary_path }}` because the
`Service` field has kind `reflect.Map of String → interface{}`,
which is the `**no**` row above. The OsSystemd handler then writes
the literal `{{ binary_path }}` to disk. systemctl fails to load
with a parse error.

## Why it's a bug, not a smell

- Reproducible.
- The user gets a confusing systemd error, far from the actual
  cause (template engine never invoked on the field).
- The fix is the same shape F019's fix took (extending the kind
  set to include `Map of String → interface{}` and unwrapping
  `reflect.Interface` to the underlying string).

## Suggested fix

Mirror F019's resolver fix. After F019's `resolveMapInPlace`
helper lands in `internal/secrets/resolver/resolve.go`, the
planner needs the equivalent. Even better: factor a shared
`internal/reflectutil` package with a `WalkStringFields(rv, fn)`
helper that both consume.

Inline-only version:

```go
// add a new case in walkAndRender's main switch
case reflect.Map:
    if fv.Type().Key().Kind() != reflect.String {
        continue
    }
    elemKind := fv.Type().Elem().Kind()
    if elemKind != reflect.String && elemKind != reflect.Interface {
        continue
    }
    for _, k := range fv.MapKeys() {
        v := fv.MapIndex(k)
        // Unwrap interface{} to its underlying value.
        if v.Kind() == reflect.Interface {
            v = v.Elem()
        }
        // Only render string-valued entries. Nested maps / lists need
        // a deeper recursion — separate finding if it surfaces.
        if v.Kind() != reflect.String {
            continue
        }
        s := v.String()
        if s == "" {
            continue
        }
        rendered, err := render(s)
        if err != nil {
            return fmt.Errorf("%s[%s]: %w", sf.Name, k.String(), err)
        }
        fv.SetMapIndex(k, reflect.ValueOf(rendered))
    }
```

Note: maps can also nest (`map[string]interface{}` whose values
are themselves maps/lists). For YAML systemd sections this is
unusual but possible. A full recursive walk over the
`interface{}` graph is the strict-correct answer; the simpler
patch above covers the **common** case (string-valued entries)
and matches what F019 settled on. Surface a TODO comment for the
recursive case and revisit if a user hits it.

## What about Pointer → Map and Slice of Struct?

Same gap exists. Lower priority because no action struct uses
those shapes today, but a future schema addition would land in
the same trap.

Add a defense-in-depth verification pass at the end of
`renderActionTemplates` that walks the rendered struct and
errors on any string containing `"{{ "` and `" }}"` — i.e.
"unrendered template syntax". That would catch every gap
(including future ones) loudly.

## Verification

- Add `TestPlanner_RendersOsSystemdServiceSection`: a step with
  `os.systemd.service.ExecStart = "{{ binary_path }}"` and a
  defined `binary_path` var. Assert the post-plan value is the
  rendered path, not the literal template.
- `go test ./internal/plan/...`
- Manual: apply a config like the repro above; verify the unit
  file contains the rendered path.

## References

- F019 — secrets-resolver version of this exact class of bug
  (fixed). The fix introduced a `resolveMapInPlace` helper —
  pattern transferable.
- `internal/plan/planner.go:782-786` — walkAndRender's existing
  comment explicitly lists the supported kinds; updating it is
  part of the fix.
