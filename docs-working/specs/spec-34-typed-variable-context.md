# Spec 34 — Typed Variable Context

**Status**: Implemented (commit `8074812`)  
**Files**: `internal/executor/scope.go` (new), `internal/executor/context.go`, `internal/executor/result.go`

---

## Problem

`Variables map[string]interface{}` is the single shared bus for all runtime state:
user-defined vars, system facts, loop context, registered results, and live metrics.
140 uses in non-test code. Type errors surface at runtime inside pongo2 or expr-lang,
not at the point of assignment.

**The five conceptually distinct categories currently collapsed into one map:**

| Category | Keys (examples) | Value types | Set by |
|---|---|---|---|
| User vars | anything | string, int, bool, list, map | `Vars` action, config `vars:` |
| Facts | `os`, `arch`, `hostname`, `cpu_cores`, … | string, int, float, []string, structs | `AddGlobalVariables` |
| Loop context | `item`, `index`, `first`, `last` | interface{}, int, bool | executor loop machinery |
| Registered results | `result_name` (step.As) | `map[string]interface{}` | `Result.RegisterTo()` |
| Live metrics | `cpu_usage_pct`, `memory_used_pct`, … | float64 | `AddGlobalVariables` |

**Concrete failure modes today:**

1. **Silent key typo** — `ec.Variables["changed_whe"]` returns nil, no error, wrong
   behavior. Happens inside type assertions (`ec.Variables["item"].(filetree.Item)`)
   and inside templates (`{{ itm }}` renders as empty string).

2. **Result shape drift** — `Result.RegisterTo()` produces a fixed key set
   (`stdout`, `stderr`, `rc`, `changed`, `failed`). Any code that accesses
   `variables["result"]["stoud"]` gets nil silently. No schema enforced at access time.

3. **Loop variable type surprise** — `item` holds `interface{}` (could be string, int,
   map, or `filetree.Item`). Accessing `.(filetree.Item)` panics if `with_items` was
   used instead of `with_filetree`. Currently guarded by two separate code paths, not
   by the type system.

4. **Facts key collisions** — a user var named `os` or `hostname` silently shadows the
   fact. Merging order determines which wins; there is no boundary enforcement.

---

## Hard Constraint

Both consumer engines accept only `map[string]interface{}`:

- **pongo2**: `pongoTemplate.Execute(map[string]interface{})`
- **expr-lang**: `expr.Env(map[string]interface{})`

A typed layer cannot change the engine APIs. It must coerce to `map[string]interface{}`
at the boundary. This caps how much compile-time safety is achievable.

---

## What The Facts System Already Shows

`Facts` is a typed struct. `facts.Collect()` returns `*Facts`, not a map.
`.ToMap()` converts at the injection boundary. This is the right pattern:
type safety in Go code, coercion only at the template/expression engine.

The variable system should extend this pattern to the other four categories.

---

## Proposal: `VariableScope` with typed sections

Replace `Variables map[string]interface{}` in `ExecutionContext` with a
`*VariableScope` struct that holds each category in its native type.

```go
// VariableScope holds all variables available to a step.
// Each category is stored in its native type; ToMap() merges them at
// the template/expression engine boundary.
type VariableScope struct {
    // User holds vars from Vars actions and config vars: blocks.
    // Untyped by necessity — values come from YAML and can be anything.
    User map[string]interface{}

    // Facts holds system facts (os, arch, hostname, cpu_cores, etc.)
    // Typed struct, set once at run start, read-only during execution.
    Facts *facts.Facts

    // Loop holds the current loop iteration state, or nil outside a loop.
    Loop *LoopContext

    // Results holds registered results keyed by step.As name.
    Results map[string]RegisteredResult

    // Metrics holds live daemon metrics (cpu_usage_pct, memory_used_pct, etc.)
    // Typed, set once at run start.
    Metrics *metrics.Metrics
}

// LoopContext holds typed loop variables — replaces the raw "item"/"index"
// string keys that required type assertions.
type LoopContext struct {
    Item  interface{}  // the current item (string or filetree.Item)
    Index int
    First bool
    Last  bool
}

// RegisteredResult is the typed shape produced by Result.RegisterTo().
// Replaces the map[string]interface{} that callers had to assert into.
type RegisteredResult struct {
    Stdout    string
    Stderr    string
    Rc        int
    Changed   bool
    Failed    bool
    Skipped   bool
    DurationMs int
    Data      map[string]interface{}  // custom data from SetData()
}
```

`ToMap()` merges all sections into `map[string]interface{}` for the engine boundary:

```go
func (v *VariableScope) ToMap() map[string]interface{} {
    m := make(map[string]interface{}, 64)
    for k, val := range v.User { m[k] = val }
    if v.Facts != nil {
        for k, val := range v.Facts.ToMap() { m[k] = val }
    }
    if v.Loop != nil {
        m["item"]  = v.Loop.Item
        m["index"] = v.Loop.Index
        m["first"] = v.Loop.First
        m["last"]  = v.Loop.Last
    }
    for k, r := range v.Results { m[k] = r.ToMap() }
    if v.Metrics != nil {
        for k, val := range v.Metrics.ToMap() { m[k] = val }
    }
    return m
}
```

**Access pattern for template/expression engines** (two sites, no change to engine API):

```go
// Before
rendered, err := ec.Svc.Template.Render(tmpl, ec.Variables)

// After
rendered, err := ec.Svc.Template.Render(tmpl, ec.Scope.ToMap())
```

**Direct loop variable access** (currently ~19 type-asserted reads):

```go
// Before — panics if wrong loop type
item := ec.Variables["item"].(filetree.Item)

// After — typed, no assertion needed
item := ec.Scope.Loop.Item.(filetree.Item)  // still needs assertion for the union
// OR: separate Item and FileTreeItem fields in LoopContext
```

**Registered result access** (currently map[string]interface{} lookups):

```go
// Before
result := ec.Variables[step.As].(map[string]interface{})
rc := result["rc"].(int)

// After
result := ec.Scope.Results[step.As]
rc := result.Rc  // typed field, no assertion
```

---

## Migration Impact

| Area | Change | Risk |
|---|---|---|
| `ExecutionContext.Variables` → `ExecutionContext.Scope` | ~140 sites | Medium — mechanical |
| `ec.Variables["item"]` → `ec.Scope.Loop.Item` | ~12 sites | Low |
| `ec.Variables["index"]` → `ec.Scope.Loop.Index` | ~7 sites | Low |
| `Result.RegisterTo()` → populate `RegisteredResult` struct | 3 call sites | Low |
| `AddGlobalVariables` → populate `Scope.Facts`, `Scope.Metrics` | 1 site | Low |
| `handleVars` → merge into `Scope.User` | 1 site | Low |
| Template/evaluator call sites → `.ToMap()` | ~30 sites | Low |
| Clone() in `ExecutionContext` | 1 site | Low |
| Test construction sites | ~40 files | Mechanical |

The largest surface is the ~140 `ec.Variables[...]` and `ec.Variables = ...` sites.
Most are in executor.go and the 28 action handlers. All are mechanical renames.

---

## What This Does NOT Fix

- `User` vars remain `map[string]interface{}` — YAML values cannot be typed at
  parse time without a schema language
- Template typos in `{{ varname }}` — pongo2 silently renders `""` for unknown keys;
  fixing this requires a different template strategy outside this spec's scope
- expr-lang type errors — still runtime; expr-lang's `AllowUndefinedVariables()` is
  already enabled, keeping the existing lenient behavior

---

## What This DOES Fix

- `Results` section: type-safe access to `rc`, `stdout`, `changed` etc. — no
  more `.(map[string]interface{})` chains
- `Loop` section: no more `.(string)` / `.(filetree.Item)` dual-path type assertions
- `Facts` and `Metrics` sections: boundaries are explicit; a user var can no longer
  silently shadow `os` or `cpu_cores` (merge order enforced in `ToMap()`)
- Adding a new variable category (e.g., `Artifacts`) requires adding a typed field,
  not documenting a string key convention

---

## Alternatives Considered

**Keep flat map, add typed accessors** (`GetString(key)`, `GetInt(key)`): provides
ergonomic reads but doesn't enforce set-time types or prevent key collisions.
Lower cost, lower value.

**Full `map[string]TypedValue` with a value union**: would catch all type errors but
requires touching both engine boundaries and every value producer. Over-engineered
for Go without sum types.

**No change**: acceptable if the codebase remains small and the variable surface doesn't
grow. The current code works correctly; this is a correctness-at-scale improvement.

---

## Recommended Phasing

**Phase 1** (low risk, immediate value): Add `RegisteredResult` struct, update
`Result.RegisterTo()`, update the 3 call sites. Zero architectural change.

**Phase 2** (medium risk): Add `LoopContext` struct, update loop injection and the
~19 type-asserted reads. Zero architectural change.

**Phase 3** (larger): Introduce `VariableScope`, wire `ExecutionContext.Scope`,
migrate all 140 sites. Run in a worktree; chase compile errors.

---

## Implementation Notes

All three phases implemented in commit `8074812`.

**Deviations from proposal:**

- `scope.go` used as the new file (not `variables.go`).
- Merge priority in `ToMap()` is `Facts < Metrics < User < Results < Loop` — user vars
  override facts (Ansible-style). The proposal draft said Facts > User but the implemented
  order matches Ansible, which is what users expect.
- Shadow warning added: `MergeUserVars()` emits `[WARNING]` via `Infof` when a user var
  key collides with a fact or metric key. Logger interface has no `Warnf`.
- `//nolint:unused` applied to `markStepFailed`, `handleVars`, `shouldSkipByTags`,
  `parseFileMode` — golangci-lint doesn't count `export_test.go` references.

**gosec fixes included in the same commit:**

- G302: lockfile permissions tightened from `0o644` to `0o600`.
- G304: added to the global gosec exclusion list in `Makefile` — mooncake is a
  file-management tool; operating on user-specified paths is intentional.
