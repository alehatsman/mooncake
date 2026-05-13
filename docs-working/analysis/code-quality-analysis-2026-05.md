# Code Quality Analysis — May 2026

Scope: full internal/ survey (~100K lines, 340 files, 28 action handlers).

---

## Dependency hygiene — clean

7 direct deps. Stdlib for HTTP, JSON, OS. External only for: YAML, template
engine (pongo2), expression evaluator (expr-lang), CLI framework (urfave/cli),
color, terminal, JSON schema validation. No ORM, no logging framework, no
metrics SDK. Nothing gratuitous.

---

## Issue 1: Step action dispatch — four parallel if-chains (HIGH)

`internal/config/config.go:700` — `Step` uses 28 nullable pointer fields as a
tagged union (no sum types in Go). The consequence is four manually-maintained
full-enumeration functions that must stay in sync:

| Function | Location | Lines |
|---|---|---|
| `countActions()` | config.go:905 | ~90 |
| `DetermineActionType()` | config.go:998 | ~90 |
| `Step.Clone()` | config.go:1121 | ~50 per-field copies |
| `renderActionTemplates()` | planner.go:649 | ~160, covers 7 of 28 actions |

Add an action: update all four. Miss one: silent bug. `renderActionTemplates`
already only covers 7 of 28 actions with no documented rationale for the
exclusions — template errors in `cmd`, `text.replace`, and 19 others surface
at execute time, not plan time.

See **spec-32** for the proposed fix.

---

## Issue 2: `map[string]interface{}` variable bus (MEDIUM)

140 uses in non-test code. The entire variable system is untyped —
`Variables map[string]interface{}` in `ExecutionContext`, threaded through
templates, evaluator, registered results, facts, metrics. Type errors surface
at runtime inside template render or `Evaluate()`, not at parse time.

The `Facts` system already returns a typed struct; `.ToMap()` converts it at
injection. That's the right boundary. The variable context itself could carry
a typed layer on top, coercing to `interface{}` only at the template engine
boundary.

Not blocking for current usage. Will hurt as the expression surface grows.

---

## Issue 3: `ExecutionContext` god struct (MEDIUM)

`internal/executor/context.go:65` — 20+ fields: file paths, display state,
execution mode, statistics, template engine, expression evaluator, path util,
file tree walker, security redactor, event publisher, sudo password, loop
state, step ID, current result.

`Clone()` at line 150 manually copies every field. `CurrentResult *Result` is
NOT in Clone() — `CurrentStepID` immediately before it IS. Whether that
omission is intentional is undocumented. Add a field, forget Clone(): nested
context runs with a zero value, no compile error.

`SudoPass string` is plaintext in a struct copied on every loop/include nesting.
The `Redactor` prevents it from appearing in logs, but the field itself is
replicated across the entire execution tree.

---

## Issue 4: Exported internal functions (MEDIUM)

`executor.go` exports: `HandleVars`, `HandleWhenExpression`, `ShouldSkipByTags`,
`CheckIdempotencyConditions`, `CheckSkipConditions`, `MarkStepFailed`,
`DispatchStepAction`, `ExecuteStep`, `ExecuteSteps`, `AddGlobalVariables`.

All are marked:

> INTERNAL: This function is exported for testing purposes only and is not
> part of the public API.

These are public API whether intended or not. Fix: unexport them, add an
`export_test.go` file that re-exports via aliases only for test packages. This
is standard Go practice for this exact situation.

---

## Issue 5: Dead dispatch comments (LOW)

`executor.go:44–46` still describes a "legacy fallback for non-migrated actions."
All 28 actions are in the registry. The fallback path just returns
`fmt.Errorf("no handler registered")`. The comment is false.

`ExecuteStep` has two plan-mode code paths: one for registered handlers (line
571) and one for unknown handlers (line 583). The "unknown" path is dead.
Both comments describe it as active behavior.

---

## Issue 6: `ShouldSkipByTags` implemented twice (LOW)

`executor.go:252` — `ShouldSkipByTags(step, ec)` (exported, executor)
`planner.go:868` — `shouldSkipByTags(stepTags, filterTags)` (unexported, planner)

Identical logic, different signatures. Divergence risk: one changes, the other
doesn't, plan and apply produce different skip behavior for the same tags.

---

## Issue 7: `formatIncludeChain` two-pass string build (LOW)

`planner.go:852` — builds a `[]string` slice, then manually concatenates with
`+=` in a loop. Should be `strings.Join(parts, " -> ")`. The `+=` pattern
allocates O(n) copies on each iteration. Irrelevant for 5-deep stacks.
Still wrong.

---

## What's actually correct

- Registry with `sync.RWMutex` — correct
- Custom error types with `errors.As`/`errors.Is` compatibility — correct  
- `Performer` interface for plan vs apply mode — clean boundary, right abstraction
- Events system — loosely coupled, no circular deps
- `panic` in `init()`-time `Register()` — acceptable, same convention as `http.Handle`
- Test coverage ~57% line ratio — above average for Go at this size
- Dependency count — minimal, principled
