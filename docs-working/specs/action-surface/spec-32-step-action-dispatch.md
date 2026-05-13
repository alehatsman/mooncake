# Spec 32: Collapse Step Action Dispatch

**Status:** Draft
**Effort:** M (1–2 weeks)
**Value:** Structural. Replaces 4 parallel if-chains with 2 mechanisms.
Fixes `ServiceAction` deep copy bug. Makes plan-time rendering correct and
complete for all actions via `RenderPreserving`.

---

## Problem

`Step` in `internal/config/config.go:700` represents its action as one of 28
nullable pointer fields — Go's closest approximation to a tagged union. This
forces four functions to independently enumerate every action field:

| Function | File | Size | What it does |
|---|---|---|---|
| `countActions()` | config.go:905 | 90 lines | nil-checks each field, returns count |
| `DetermineActionType()` | config.go:998 | 90 lines | nil-checks each field, returns YAML key string |
| `Clone()` | config.go:1121 | 50 lines | copies all fields into new Step literal |
| `renderActionTemplates()` | planner.go:649 | 160 lines | deep-copies + renders plan-time templates |

**Adding a new action requires updating all four.** Missing any one:

- Miss `countActions` → multi-action validation silently passes
- Miss `DetermineActionType` → type returns `"unknown"`, handler lookup fails at
  runtime with a generic error
- Miss `Clone` → loop/include expansion runs a nil action; error appears as
  "step has no action" deep in the executor, not at the missing action's callsite
- Miss `renderActionTemplates` → plan output shows raw template strings for that action

### Current `renderActionTemplates` has two problems

**Problem 1 — coverage:** Only 7 of 28 actions have entries. The other 21
are silently skipped. There is no way to distinguish "deliberately omitted"
from "forgotten when the function was written."

**Problem 2 — correctness:** `renderActionTemplates` calls `p.template.Render()`
directly. pongo2 renders undefined variables as empty string silently. This
means templates that reference variables registered by previous steps at
execute time — `{{ registered_result.stdout }}` — get clobbered at plan time.
The `{{ }}` markers are replaced with `""`. At execute time, the handler
re-renders the string, but the markers are gone, so the variable is never
substituted. This is a silent data loss bug for any template field that
references an execute-time variable.

### Additional bug: `ServiceAction` deep copy is shallow

`renderActionTemplates` (planner.go:771):

```go
serviceCopy := *step.OsService   // shallow copy — Unit and Dropin pointers shared
step.OsService = &serviceCopy
serviceCopy.Unit.SrcTemplate = rendered   // writes through to original ServiceUnit
```

`ServiceAction.Unit` and `ServiceAction.Dropin` are nested pointers. The
shallow copy does not deep-copy them. Writing to `serviceCopy.Unit.SrcTemplate`
mutates the original config's `ServiceUnit`.

---

## Design

Three components, introduced in order.

---

### Component 0 — `RenderPreserving` (prerequisite, in `internal/template/`)

The root cause of the correctness bug: pongo2 has no built-in "preserve
unknown variables" mode, and its `{% verbatim %}`/`{% raw %}` tags are not
implemented in pongo2 v6. The fix lives in the renderer layer.

**Mechanism: sentinel substitution.** Before rendering, scan the template for
`{{ expr }}` blocks whose root variable is not in `vars`. Replace each with a
unique opaque sentinel string. Render the modified template — defined variables
resolve normally, sentinels pass through as plain text. After rendering,
replace sentinels back with the original `{{ expr }}` syntax.

```
input:   "{{ preset_dir }}/{{ registered_result.path }}/config.j2"
vars:    { preset_dir: "/home/user/.mooncake" }
step 1:  "{{ preset_dir }}/__PRSV_0__"
step 2:  "/home/user/.mooncake/__PRSV_0__"           (pongo2 renders)
output:  "/home/user/.mooncake/{{ registered_result.path }}/config.j2"
```

At execute time the handler calls the existing `Render` (not `RenderPreserving`)
with the full variable scope including `registered_result`. The preserved
`{{ }}` markers render correctly then.

**Add `RenderPreserving` to the `Renderer` interface:**

```go
// internal/template/renderer.go

type Renderer interface {
    Render(template string, variables map[string]interface{}) (string, error)

    // RenderPreserving renders like Render but preserves {{ expr }} placeholders
    // for any root variable not present in variables, rather than silently
    // substituting empty string. Use this for plan-time rendering where
    // execute-time variables (registered results from previous steps) are
    // not yet in scope.
    RenderPreserving(template string, variables map[string]interface{}) (string, error)
}
```

**Implementation in `Pongo2Renderer`:**

```go
var (
    // matches {{ expr }} blocks; excludes {% %} tags via [^%{]
    templateExprRe = regexp.MustCompile(`\{\{[^%{][^}]*\}\}`)
    // extracts root identifier from {{ expr }}
    rootVarRe      = regexp.MustCompile(`\{\{-?\s*([a-zA-Z_][a-zA-Z0-9_]*)`)
    // pongo2 built-ins that live in its private context, not in user vars
    pongo2Builtins = map[string]bool{
        "true": true, "false": true, "none": true, "pongo2": true,
    }
)

func (r *Pongo2Renderer) RenderPreserving(tmpl string, vars map[string]interface{}) (string, error) {
    type entry struct{ placeholder, original string }
    var sentinels []entry
    n := 0

    modified := templateExprRe.ReplaceAllStringFunc(tmpl, func(expr string) string {
        m := rootVarRe.FindStringSubmatch(expr)
        if len(m) < 2 || pongo2Builtins[m[1]] {
            return expr
        }
        if _, ok := vars[m[1]]; ok {
            return expr // variable in scope — render normally
        }
        ph := fmt.Sprintf("__PRSV_%d__", n)
        n++
        sentinels = append(sentinels, entry{ph, expr})
        return ph
    })

    rendered, err := r.Render(modified, vars)
    if err != nil {
        return "", err
    }
    for _, s := range sentinels {
        rendered = strings.ReplaceAll(rendered, s.placeholder, s.original)
    }
    return rendered, nil
}
```

**Scope of `RenderPreserving`:** used only in the plan-time render closure
passed to `walkAndRender`. Execute-time handlers call `Render` — at execute
time all referenced variables should be in scope, and an undefined variable
is a user error, not a deferred reference.

---

### Component 1 — `action:` struct tags drive `countActions` and `DetermineActionType`

Add an `action:` struct tag to each of the 28 action fields in `Step`:

```go
// internal/config/config.go

type Step struct {
    // ...
    // Field declaration order determines DetermineActionType priority.
    FileWrite        *File             `yaml:"file.write"     json:"file.write,omitempty"     action:"file.write"`
    FileTemplate     *Template         `yaml:"file.template"  json:"file.template,omitempty"  action:"file.template"`
    FileCopy         *Copy             `yaml:"file.copy"      json:"file.copy,omitempty"      action:"file.copy"`
    Shell            *ShellAction      `yaml:"shell"          json:"shell,omitempty"           action:"shell"`
    Cmd              *CommandAction    `yaml:"cmd"            json:"cmd,omitempty"             action:"cmd"`
    // ... all 28 action fields; ForEach/ForEachFile/Name/When/etc. get NO tag
}
```

Derive `countActions` and `DetermineActionType` from reflection over those tags:

```go
var stepType = reflect.TypeOf(Step{})

// actionFieldIndices is computed once: struct field indices of all fields
// with an `action:` tag, in declaration order.
var actionFieldIndices = func() []int {
    var idx []int
    for i := 0; i < stepType.NumField(); i++ {
        if _, ok := stepType.Field(i).Tag.Lookup("action"); ok {
            idx = append(idx, i)
        }
    }
    return idx
}()

// ActionFieldIndices returns the cached action field indices for use by
// the planner's renderActionTemplates.
func ActionFieldIndices() []int { return actionFieldIndices }

func (s *Step) countActions() int {
    rv := reflect.ValueOf(s).Elem()
    n := 0
    for _, i := range actionFieldIndices {
        if !rv.Field(i).IsNil() {
            n++
        }
    }
    return n
}

func (s *Step) DetermineActionType() string {
    rv := reflect.ValueOf(s).Elem()
    for _, i := range actionFieldIndices {
        if !rv.Field(i).IsNil() {
            return stepType.Field(i).Tag.Get("action")
        }
    }
    // Defensive: for_each/for_each_file without an action.
    // Planner expands loops before compilePlanStep, so this should not
    // appear in a fully-built plan.
    if s.ForEach != nil || s.ForEachFile != nil {
        return "loop"
    }
    return "unknown"
}
```

`actionFieldIndices` is computed once at package init via an IIFE. Each
`countActions`/`DetermineActionType` call iterates ≤28 pre-computed indices
calling `reflect.Value.IsNil()`. Cost: ~200–400 ns per call. Not a hot path.

`Vars` (`*map[string]interface{}`), `VarsLoad` (`*string`), `Import` (`*string`)
are non-struct pointers. They still get `action:` tags and are nil-checked
identically — `reflect.Value.IsNil()` works for any pointer kind.

---

### Component 2 — Generic string-field walker replaces `renderActionTemplates`

**Design principle (from Terraform):** Plan output should distinguish "known now" from "known after execution" — never show empty string for a missing variable. With `RenderPreserving` in place, we can walk every string field of every action struct automatically. No per-action registration, no opt-in interface, no boilerplate.

**`plan:"path"` struct tag** marks fields that hold file paths and need relative→absolute resolution at plan time. This is required for executor correctness: the executor receives compiled plan steps with no knowledge of the source config file's directory. A relative path in a compiled step cannot be resolved at execute time.

```go
// Example tags — applied to path fields across all action structs
type Template struct {
    Src  string `yaml:"src"  json:"src"  plan:"path"`
    Dest string `yaml:"dest" json:"dest" plan:"path"`
    Mode string `yaml:"mode" json:"mode,omitempty"`    // render only, not a path
}

type Copy struct {
    Src  string `yaml:"src"  json:"src"  plan:"path"`
    Dest string `yaml:"dest" json:"dest" plan:"path"`
    // ...
}
```

Fields tagged `plan:"path"` are: rendered with `RenderPreserving`, then `filepath.Join(currentDir, rendered)` applied if the result is not already absolute.

**`walkAndRender`** — package-level function in `internal/plan/`:

```go
// walkAndRender recursively renders all string fields of an action struct
// using RenderPreserving. Fields tagged plan:"path" are additionally resolved
// to absolute paths using currentDir. Nested pointer-to-struct fields are
// deep-copied before mutation to avoid touching the original config.
func walkAndRender(rv reflect.Value, render func(string) (string, error), currentDir string) error {
    rt := rv.Type()
    for i := 0; i < rv.NumField(); i++ {
        fv := rv.Field(i)
        sf := rt.Field(i)
        switch fv.Kind() {
        case reflect.String:
            if fv.String() == "" { continue }
            rendered, err := render(fv.String())
            if err != nil { return fmt.Errorf("%s: %w", sf.Name, err) }
            if sf.Tag.Get("plan") == "path" && !filepath.IsAbs(rendered) {
                rendered = filepath.Join(currentDir, rendered)
            }
            fv.SetString(rendered)
        case reflect.Ptr:
            if fv.IsNil() { continue }
            switch fv.Type().Elem().Kind() {
            case reflect.String:
                if fv.Elem().String() == "" { continue }
                rendered, err := render(fv.Elem().String())
                if err != nil { return fmt.Errorf("%s: %w", sf.Name, err) }
                if sf.Tag.Get("plan") == "path" && !filepath.IsAbs(rendered) {
                    rendered = filepath.Join(currentDir, rendered)
                }
                cp := reflect.New(fv.Type().Elem())
                cp.Elem().SetString(rendered)
                fv.Set(cp)
            case reflect.Struct:
                orig := fv.Elem()
                cp := reflect.New(orig.Type())
                cp.Elem().Set(orig)
                fv.Set(cp)
                if err := walkAndRender(cp.Elem(), render, currentDir); err != nil { return err }
            }
        case reflect.Slice:
            if fv.Type().Elem().Kind() != reflect.String { continue }
            for j := 0; j < fv.Len(); j++ {
                if fv.Index(j).String() == "" { continue }
                rendered, err := render(fv.Index(j).String())
                if err != nil { return fmt.Errorf("%s[%d]: %w", sf.Name, j, err) }
                fv.Index(j).SetString(rendered)
            }
        case reflect.Map:
            if fv.Type().Key().Kind() != reflect.String || fv.Type().Elem().Kind() != reflect.String { continue }
            for _, k := range fv.MapKeys() {
                if fv.MapIndex(k).String() == "" { continue }
                rendered, err := render(fv.MapIndex(k).String())
                if err != nil { return fmt.Errorf("%s[%s]: %w", sf.Name, k.String(), err) }
                fv.SetMapIndex(k, reflect.ValueOf(rendered))
            }
        }
    }
    return nil
}
```

Handles: `string`, `*string`, `*struct` (with deep copy + recursion), `[]string`, `map[string]string`. Skips everything else silently.

**New `renderActionTemplates`:**

```go
func (p *Planner) renderActionTemplates(step *config.Step, ctx *ExpansionContext) error {
    render := func(s string) (string, error) {
        return p.template.RenderPreserving(s, ctx.Variables)
    }
    rv := reflect.ValueOf(step).Elem()
    for _, i := range config.ActionFieldIndices() {
        fv := rv.Field(i)
        if fv.IsNil() { continue }
        if fv.Type().Elem().Kind() != reflect.Struct { continue }
        // Shallow-copy the action struct. walkAndRender deep-copies nested
        // pointer fields before mutating them.
        orig := fv.Elem()
        cp := reflect.New(orig.Type())
        cp.Elem().Set(orig)
        fv.Set(cp)
        if err := walkAndRender(cp.Elem(), render, ctx.CurrentDir); err != nil {
            return fmt.Errorf("step %q: %w", step.Name, err)
        }
        break
    }
    return nil
}
```

No `PlanRenderer` interface check. Every struct action is walked. Coverage is automatic and complete for all 28 actions.

**`PlanRenderer` interface deleted.** `internal/config/plan_renderer.go` removed. The 6 manual implementations (`ShellAction.PlanRenderInPlace`, `File.PlanRenderInPlace`, etc.) are gone.

**Fields with `plan:"path"` tag (complete list):**

| Struct | Fields |
|---|---|
| `File` | `Path`, `Src` |
| `Template` | `Src`, `Dest` |
| `Copy` | `Src`, `Dest` |
| `Unarchive` | `Src`, `Dest`, `Creates` |
| `Download` | `Dest` |
| `ServiceUnit` | `Dest`, `SrcTemplate` |
| `ServiceDropin` | `SrcTemplate` |
| `FileReplace` | `Path` |
| `FileInsert` | `Path` |
| `FileDeleteRange` | `Path` |
| `FilePatchApply` | `Path`, `PatchFile` |
| `RepoSearch` | `Path`, `OutputFile` |
| `RepoTree` | `Path`, `OutputFile` |
| `RepoApplyPatchset` | `PatchsetFile`, `BaseDir`, `OutputFile` |
| `ArtifactCapture` | `OutputDir` |
| `ArtifactValidate` | `ArtifactFile` |
| `WaitAction` | `Path` (`*string`) |
| `AssertFile` | `Path` |
| `AssertFileSHA256` | `Path` |

### `Clone()` — no change

`Clone()` at config.go:1121 stays as a struct literal. `go vet` produces a
compile error if a new Step field is added but not included. This is the
correct enforcement mechanism — no improvement needed.

---

## What changes per function

| Function | Before | After |
|---|---|---|
| `countActions` | 90-line if-chain | 10-line reflection loop |
| `DetermineActionType` | 90-line if-chain | 10-line reflection loop |
| `Clone` | struct literal | **unchanged** |
| `renderActionTemplates` | 160-line if-chain, 7/28 covered, silent clobber bug, OsService shallow copy bug | generic recursive walker, all 28 actions covered, `plan:"path"` tag drives path resolution, `PlanRenderer` interface deleted |

---

## What "add a new action" looks like after

1. Add field to `Step` with `action:"key"` tag → `countActions` and
   `DetermineActionType` automatically correct.
2. Add field to `Clone()` struct literal → compile error if missed.
3. Add `plan:"path"` tags to any path fields on the action struct →
   `walkAndRender` handles rendering and path resolution automatically.
   No other registration or boilerplate required.

---

## Design implications of `RenderPreserving`

`RenderPreserving` changes what plan-time rendering can safely do.

**Before `RenderPreserving`:**
Plan-time rendering was only safe for fields that reference variables
guaranteed to exist at plan time (loop variables, `vars:` declarations). Any
field that might reference an execute-time registered variable had to be left
out. The 21 unhandled actions were "correctly skipped" because their fields
might reference execute-time variables.

**After `RenderPreserving`:**
There is no unsafe plan-time rendering. Undefined variables are preserved.
Every action's string fields can be rendered at plan time without risk.

**This means the 21 "skipped" actions are no longer correctly skipped — they
are merely not yet covered.** The distinction matters:

- Previously: rendering `cmd` fields would be wrong (might clobber `{{ registered.stdout }}`)
- After: `walkAndRender` covers `cmd` and every other action automatically — better plan output, zero correctness risk

**Coverage strategy after this spec:**

`walkAndRender` covers all 28 actions automatically — no per-action work
required. Path fields that need absolute resolution get `plan:"path"` tags;
all other string fields are rendered with `RenderPreserving`. The ceiling is
reached immediately: all 28 actions render all their string fields at plan
time. Users see fully-resolved plan output where variables are in scope, and
preserved `{{ expr }}` where they are not. This is strictly better than the
current state for every action.

**Execute-time rendering is unchanged.** Handlers call `Render` (not
`RenderPreserving`). At execute time, all variables should be in scope. If a
variable is missing at execute time, empty substitution is the correct
behavior — it is a user configuration error, not a deferred reference.

---

## Migration steps

**Step 0** — Implement `RenderPreserving` on `Pongo2Renderer`. Add to the
`Renderer` interface. Write unit tests: defined vars render normally, undefined
vars are preserved, mixed templates partially render. This step is completely
independent of everything else in this spec.

**Step 1** — Add `action:"key"` tags to all 28 action fields in `Step`. No
code changes elsewhere. Run `go build`.

**Step 2** — Add `actionFieldIndices` cache. Replace `countActions` and
`DetermineActionType` with reflection loops. Delete the old if-chains.
Run `go test ./...`. Add a table-driven test that sets each action field on a
`Step` and verifies `DetermineActionType()` returns the tag value.

**Step 3** — Add `plan:"path"` struct tags to all path fields listed in the
table above (across `internal/config/config.go`). No behavior change yet —
tags are inert until Step 4.

**Step 4** — Replace `renderActionTemplates` body with `walkAndRender`. Add
`walkAndRender` as a package-level function in `internal/plan/planner.go`.
Delete `internal/config/plan_renderer.go`. Run `go test ./...`.

**Step 5** — Clean up `//nolint:gocyclo` and `//nolint:dupl` suppressions
on the deleted if-chains.

Steps 0–2 are completely independent. Steps 3–4 depend on Step 0 (need
`RenderPreserving`). Step 3 can merge before Step 4.

---

## Acceptance criteria

- [ ] `RenderPreserving`: defined vars render, undefined vars preserved as `{{ expr }}`, mixed templates work, pongo2 builtins not intercepted
- [ ] `DetermineActionType` returns the `action:` tag value for all 28 fields (table-driven test)
- [ ] `countActions` correctly counts all 28 field types
- [ ] `renderActionTemplates` uses `walkAndRender` with `RenderPreserving`; all 28 action structs covered automatically
- [ ] `plan:"path"` fields are resolved to absolute paths; non-path string fields are rendered only
- [ ] `ServiceAction` nested structs (`Unit`, `Dropin`) are deep-copied before mutation
- [ ] No `PlanRenderer` interface or implementations remain
- [ ] `go test ./...` passes
- [ ] No `//nolint:gocyclo` or `//nolint:dupl` remaining on the deleted functions
