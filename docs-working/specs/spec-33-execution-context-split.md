# Spec 33 — Split ExecutionContext into scope + services

**Status**: Draft  
**File**: `internal/executor/context.go`

---

## Problem

`ExecutionContext` is a 20-field god struct mixing four unrelated concerns:

| Concern | Fields |
|---|---|
| Per-scope mutable state | `Variables`, `CurrentDir`, `PresetBaseDir`, `CurrentFile`, `Level`, `CurrentIndex`, `TotalSteps`, `CurrentStepID`, `CurrentResult` |
| Shared services (constructed once, never mutate) | `Template`, `Evaluator`, `PathUtil`, `FileTree`, `Redactor`, `EventPublisher`, `Logger`, `Stats` |
| Run configuration (set at startup, read-only) | `CurrentMode`, `Tags`, `SudoPass` |

Three concrete defects follow from this mixing:

### 1. `Clone()` correctness is not enforced

`Clone()` omits `CurrentResult` and `CurrentStepID`. These are intentionally step-local
(they reset per step, not per scope), but there is no structural signal separating
"fields that Clone() should copy" from "fields it should not." Adding a new field
requires remembering to update Clone() and to decide which category it belongs to.
There is no compile error on omission.

**Additional finding**: `Clone()` is never called in production code — only in tests.
Nested scopes (includes, loops) pass `ec *ExecutionContext` by pointer directly and
mutate shared fields in-place. The Clone() contract is therefore untested against
real behavior and may drift silently.

### 2. `SudoPass` is plaintext in a frequently-copied struct

`SudoPass string` sits alongside display counters and logging state. Any
`fmt.Sprintf("%+v", ec)` — common in debug code — prints the password. The
`Redactor` exists to guard logs, but it cannot guard direct struct formatting.

### 3. 20-field struct is opaque at construction sites

`ExecutionContext{...}` in `ExecutePlan` is a 20-line struct literal.
Every new shared service or per-scope field is added to the same flat list.
There is no way to look at the struct and know which fields are shared pointers
and which are per-scope values without reading all 20 field comments.

---

## Proposal

Extract a `RunServices` struct for the shared, immutable-after-construction part.
`ExecutionContext` shrinks to per-scope mutable state plus a `*RunServices` pointer.

```go
// RunServices holds the shared, immutable-after-construction services and
// configuration for a mooncake run. One instance is created per run and
// referenced by all nested ExecutionContexts via pointer.
type RunServices struct {
    Template       template.Renderer
    Evaluator      expression.Evaluator
    PathUtil       *pathutil.PathExpander
    FileTree       *filetree.Walker
    Redactor       *security.Redactor
    EventPublisher events.Publisher
    Logger         logger.Logger
    Stats          *ExecutionStats
    Mode           actions.Mode
    Tags           []string
    SudoPass       string
}

// ExecutionContext holds per-scope state for a step sequence.
// Cloned when entering nested scopes (includes, loops); services are shared.
type ExecutionContext struct {
    Svc           *RunServices
    Variables     map[string]interface{}
    CurrentDir    string
    PresetBaseDir string
    CurrentFile   string
    Level         int
    CurrentIndex  int
    TotalSteps    int
    CurrentStepID string
    CurrentResult *Result
}
```

`Clone()` becomes structurally correct by construction:

```go
func (ec *ExecutionContext) Clone() ExecutionContext {
    vars := make(map[string]interface{}, len(ec.Variables))
    for k, v := range ec.Variables {
        vars[k] = v
    }
    return ExecutionContext{
        Svc:           ec.Svc,  // shared pointer — intentional
        Variables:     vars,
        CurrentDir:    ec.CurrentDir,
        PresetBaseDir: ec.PresetBaseDir,
        CurrentFile:   ec.CurrentFile,
        Level:         ec.Level,
        // CurrentStepID and CurrentResult intentionally omitted — reset per step
    }
}
```

Adding a new shared service → add to `RunServices` → Clone is auto-correct.  
Adding a new scope field → add to `ExecutionContext` → must update Clone() → struct literal at Clone() site gives compile error if missed.

---

## Impact

### Call sites: `ec.Template` → `ec.Svc.Template`

Every field access on a shared service changes. Quick audit:

```
grep -rn "ec\.Template\|ec\.Evaluator\|ec\.PathUtil\|ec\.FileTree\|ec\.Redactor\|ec\.EventPublisher\|ec\.Logger\|ec\.Stats\|ec\.SudoPass\|ec\.Tags\|ec\.CurrentMode" internal/
```

Estimated ~200–300 access sites across `executor/`, `actions/`.

The `actions.Context` getter methods (`GetTemplate()`, `GetLogger()`, etc.) in
`context.go` become one-line delegates to `ec.Svc.*`. No change to the
`actions.Context` interface or any handler.

### `Effects()` method

```go
func (ec *ExecutionContext) Effects() actions.Performer {
    return effects.NewPerformer(ec.Mode, ec.Svc.SudoPass)
}

func (ec *ExecutionContext) Mode() actions.Mode {
    return ec.Svc.Mode
}
```

### Construction in `ExecutePlan`

```go
svc := &RunServices{
    Template:       renderer,
    Evaluator:      evaluator,
    PathUtil:       pathExpander,
    FileTree:       fileTreeWalker,
    Redactor:       redactor,
    EventPublisher: publisher,
    Logger:         log,
    Stats:          NewExecutionStats(),
    Mode:           mode,
    SudoPass:       sudoPass,
    Tags:           []string{},
}
ec := ExecutionContext{
    Svc:        svc,
    Variables:  variables,
    CurrentDir: configDir,
    Level:      0,
    TotalSteps: len(steps),
}
```

---

## What this does NOT change

- The `actions.Context` interface — no handler changes needed
- Execution behavior — pure structural refactor
- `ExecutionStats` — stays as-is (`Stats` moves to `RunServices`)
- `EmitEvent` helper — moves to `ec.Svc.EventPublisher` internally, signature unchanged

---

## What this does NOT fix

- `map[string]interface{}` variable bus (Issue 2 — separate spec)
- The fact that `Clone()` is never called in production code (separate cleanup)

---

## Alternatives considered

**Embedded `*RunServices`**: `ec.Template` still works via promotion. Rejected because
it hides whether `Template` is scope-local or shared — the whole point is to make
that distinction visible at the call site.

**Keep flat struct, add constructor**: A `NewExecutionContext(svc, ...)` constructor
enforces all fields are set. Does not fix Clone() safety or the plaintext SudoPass
concern. Smaller change, less value.

---

## Migration steps

1. Add `RunServices` struct to `context.go`
2. Update `ExecutionContext` to `Svc *RunServices` + scope fields only
3. Update `Clone()` 
4. Update `context.go` getter methods (`GetTemplate()` etc.) to delegate to `ec.Svc.*`
5. Update `Effects()` and `Mode()`
6. Update `ExecutePlan` constructor site
7. Fix all `ec.Template` → `ec.Svc.Template` access sites (~200–300, mechanical sed)
8. Fix test construction sites (`ExecutionContext{...}` in `*_test.go`)
9. Run tests

Step 7 is the bulk of the work but is purely mechanical — no logic changes.
