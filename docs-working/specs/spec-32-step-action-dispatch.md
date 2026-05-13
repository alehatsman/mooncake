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
passed to `PlanRenderInPlace`. Execute-time handlers call `Render` — at execute
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

### Component 2 — `PlanRenderer` interface drives `renderActionTemplates`

**What plan-time rendering is for:** `compilePlanStep` is called on every
step. It calls `renderActionTemplates` so that plan output shows resolved
values — file paths with variables substituted, loop items expanded — instead
of raw `{{ }}` strings. It is NOT about correctness at execute time (handlers
re-render at execute time); it is about display quality and early syntax-error
detection.

**With `RenderPreserving` in place,** all 28 actions can safely implement
plan-time rendering. Templates referencing execute-time variables are
preserved as `{{ }}` in plan output and resolved correctly at execute time.
The restriction "only render fields the planner structurally needs" no longer
applies.

Add an optional interface to `internal/config/`:

```go
// PlanRenderer is implemented by action structs whose string fields should
// be resolved at plan time for plan output display. The method is called on
// an already-deep-copied action struct — implementations may mutate the
// receiver in place. Implementations with nested pointer fields must
// deep-copy those fields themselves before mutating (see ServiceAction).
//
// render wraps RenderPreserving: undefined variables are preserved as
// {{ expr }} rather than replaced with empty string.
//
// currentDir is the config file's directory for relative-to-absolute
// path resolution: if !filepath.IsAbs(p) { p = filepath.Join(currentDir, p) }
//
// Implementing types (keep current):
//   ShellAction, File, Template, Copy, Unarchive, ServiceAction
type PlanRenderer interface {
    PlanRenderInPlace(render func(string) (string, error), currentDir string) error
}
```

Implementing `PlanRenderer` is opt-in. No registration. Not implementing it
is unambiguous: plan output shows the raw template string for that action's
fields. This is correct behavior — not a bug.

**Action struct implementations** (same 6 as current `renderActionTemplates`
coverage, ServiceAction fixed):

```go
func (a *ShellAction) PlanRenderInPlace(render func(string) (string, error), _ string) error {
    cmd, err := render(a.Cmd)
    if err != nil { return fmt.Errorf("shell.cmd: %w", err) }
    a.Cmd = cmd
    return nil
}

func (a *File) PlanRenderInPlace(render func(string) (string, error), dir string) error {
    path, err := render(a.Path)
    if err != nil { return fmt.Errorf("file.write.path: %w", err) }
    a.Path = path
    if a.Content != "" {
        content, err := render(a.Content)
        if err != nil { return fmt.Errorf("file.write.content: %w", err) }
        a.Content = content
    }
    if a.Src != "" {
        src, err := render(a.Src)
        if err != nil { return fmt.Errorf("file.write.src: %w", err) }
        if !filepath.IsAbs(src) { src = filepath.Join(dir, src) }
        a.Src = src
    }
    return nil
}

func (a *Template) PlanRenderInPlace(render func(string) (string, error), dir string) error {
    src, err := render(a.Src)
    if err != nil { return fmt.Errorf("file.template.src: %w", err) }
    if !filepath.IsAbs(src) { src = filepath.Join(dir, src) }
    a.Src = src
    dest, err := render(a.Dest)
    if err != nil { return fmt.Errorf("file.template.dest: %w", err) }
    a.Dest = dest
    return nil
}

// Copy and Unarchive follow the same pattern as Template (src+dest with path resolution).

// ServiceAction — fixes the nested pointer bug
func (a *ServiceAction) PlanRenderInPlace(render func(string) (string, error), dir string) error {
    if a.Unit != nil && a.Unit.SrcTemplate != "" {
        unitCopy := *a.Unit        // deep copy — fixes shallow copy bug
        a.Unit = &unitCopy
        rendered, err := render(unitCopy.SrcTemplate)
        if err != nil { return fmt.Errorf("os.service.unit.src_template: %w", err) }
        if !filepath.IsAbs(rendered) { rendered = filepath.Join(dir, rendered) }
        a.Unit.SrcTemplate = rendered
    }
    if a.Dropin != nil && a.Dropin.SrcTemplate != "" {
        dropinCopy := *a.Dropin
        a.Dropin = &dropinCopy
        rendered, err := render(dropinCopy.SrcTemplate)
        if err != nil { return fmt.Errorf("os.service.dropin.src_template: %w", err) }
        if !filepath.IsAbs(rendered) { rendered = filepath.Join(dir, rendered) }
        a.Dropin.SrcTemplate = rendered
    }
    return nil
}
```

**New `renderActionTemplates` in planner** — uses `RenderPreserving`:

```go
func (p *Planner) renderActionTemplates(step *config.Step, ctx *ExpansionContext) error {
    // render wraps RenderPreserving: undefined variables are preserved as
    // {{ expr }} placeholders rather than silently replaced with "".
    render := func(s string) (string, error) {
        return p.template.RenderPreserving(s, ctx.Variables)
    }

    rv := reflect.ValueOf(step).Elem()
    for _, i := range config.ActionFieldIndices() {
        fv := rv.Field(i)
        if fv.IsNil() {
            continue
        }
        if fv.Type().Elem().Kind() != reflect.Struct {
            continue // *string/*map fields (import, vars.load, vars) — no plan-time rendering
        }
        renderer, ok := fv.Interface().(config.PlanRenderer)
        if !ok {
            continue // action does not implement PlanRenderer — plan output shows raw template
        }
        // Generic shallow deep-copy of the action struct.
        // PlanRenderInPlace implementations with nested pointers (ServiceAction)
        // are responsible for deep-copying their nested fields themselves.
        orig := fv.Elem()
        cp := reflect.New(orig.Type())
        cp.Elem().Set(orig)
        fv.Set(cp)

        if err := cp.Interface().(config.PlanRenderer).PlanRenderInPlace(render, ctx.CurrentDir); err != nil {
            return fmt.Errorf("step %q: %w", step.Name, err)
        }
        break // exactly one action field is non-nil per valid step
    }
    return nil
}
```

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
| `renderActionTemplates` | 160-line if-chain, 7/28 covered, silent clobber bug, OsService shallow copy bug | generic loop, coverage explicit, `RenderPreserving` fixes clobber, OsService fixed |

---

## What "add a new action" looks like after

1. Add field to `Step` with `action:"key"` tag → `countActions` and
   `DetermineActionType` automatically correct.
2. Add field to `Clone()` struct literal → compile error if missed.
3. Optionally implement `PlanRenderInPlace` on the action struct for better
   plan output. No consequence if omitted — plan output shows raw templates.

---

## Design implications of `RenderPreserving`

`RenderPreserving` changes the nature of `PlanRenderer` in a fundamental way.

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
are merely not yet implemented.** The distinction matters:

- Previously: adding `PlanRenderInPlace` to `cmd` would be wrong (might clobber `{{ registered.stdout }}`)
- After: adding `PlanRenderInPlace` to `cmd` is purely additive — better plan output, zero correctness risk

**Coverage strategy after this spec:**

The 6 actions implemented here match current `renderActionTemplates` coverage
— they are the actions whose path fields affect plan output most visibly.
The remaining 22 can be added incrementally, one struct at a time, with no
risk and no dependencies. Each addition is isolated: add the method, add a
test, done. There is no wrong time to add one.

The ceiling is full coverage: all 28 actions render all their string fields
at plan time. Users see fully-resolved plan output where variables are in
scope, and preserved `{{ expr }}` where they are not. This is strictly better
than the current state for every action.

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

**Step 3** — Add `PlanRenderer` interface to `internal/config/`. Implement
`PlanRenderInPlace` on `ShellAction`, `File`, `Template`, `Copy`, `Unarchive`,
`ServiceAction`. These match current coverage; no behavior change yet.

**Step 4** — Replace `renderActionTemplates` body with the generic reflection
loop using `RenderPreserving`. Run `go test ./...`. Existing tests for the 6
converted actions verify correctness.

**Step 5** — Clean up `//nolint:gocyclo` and `//nolint:dupl` suppressions
on the deleted if-chains.

Steps 0–2 are completely independent. Steps 3–4 depend on Step 0 (need
`RenderPreserving`). Step 3 can merge before Step 4.

---

## Acceptance criteria

- [ ] `RenderPreserving`: defined vars render, undefined vars preserved as `{{ expr }}`, mixed templates work, pongo2 builtins not intercepted
- [ ] `DetermineActionType` returns the `action:` tag value for all 28 fields (table-driven test)
- [ ] `countActions` correctly counts all 28 field types
- [ ] `renderActionTemplates` uses `RenderPreserving` in the render closure
- [ ] `ServiceAction` deep copy: original `ServiceUnit` / `ServiceDropin` structs not mutated after plan compile
- [ ] `go test ./...` passes
- [ ] No `//nolint:gocyclo` or `//nolint:dupl` remaining on the deleted functions
