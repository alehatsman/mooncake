---
id: F033
title: Standalone vars action leaks `!secret` sentinel — planner pre-evaluates vars at plan time, bypassing resolver.Resolve
severity: bug
package: internal/plan
file: internal/plan/planner.go
lines: 325-329, 599-620
status: open
verified: 2026-05-16 on master @ f2c6a28 (post-F019 merge)
---

## What

`resolver.Resolve` is called from exactly one place in the codebase
— `executor.dispatchRunner` (`executor/executor.go:1253`). That is
the right place for action-typed steps (file, shell, service, …)
because every action handler reaches it.

But the standalone **vars action** never reaches `dispatchRunner`:
the planner intercepts it at plan time. From
`internal/plan/planner.go:324-330`:

```go
// Handle variable operations (skip if when condition is false at plan time)
if step.Vars != nil {
    if !p.shouldProcessAtPlanTime(step, ctx) {
        return nil
    }
    return p.expandVars(step, ctx)
}
```

`expandVars` (line 599-620) iterates `step.Vars` and renders each
string value as a template, then merges into `ctx.Variables`. It
never calls `resolver.Resolve` or `security.IsMarker`. The
sentinel marker `__MOONCAKE_SECRET_v1_DO_NOT_EDIT__:env:FOO`
passes straight through:

```go
if strVal, ok := v.(string); ok {
    rendered, err := p.template.Render(strVal, ctx.Variables)
    // sentinel has no {{...}} so Render returns it verbatim
    ctx.Variables[k] = rendered          // ← marker stored as-is
}
```

`internal/actions/vars/handler.go:110-113` even foreshadows this
gap in its own doc-comment:

> Note: the planner already evaluates vars at plan time and
> strips them from the step list, so this handler rarely runs in
> practice. Run is here for completeness and to satisfy the
> Runner contract.

So the vars action's `Run` (where `dispatchRunner` would have
called Resolve) is effectively dead code on the standalone vars
path.

## F019 context

F019 fixed `resolver.Resolve` to handle `*map[string]interface{}`
and `map[string]interface{}` inside Step structs. The regression
test (`TestResolve_StepVarsContainsMarker`) calls `Resolve(step)`
directly and asserts the map is mutated in place. That test
passes — the resolver itself is correct.

**The fix is in the right shape for the wrong call site.** F019
helps for any action whose handler has a `map[string]interface{}`
field that reaches `dispatchRunner`. But the headline case
implied by F019's title — `step.Vars` of the vars action — is
intercepted before `dispatchRunner` is ever called.

## Verified repro (post-F019 merge, master @ f2c6a28)

```yaml
# /tmp/f025-repro.yml
steps:
  - name: stash secret in user vars
    vars:
      INNER_TOKEN: !secret env:F025_REPRO_TOKEN

  - name: use the stashed secret
    shell:
      cmd: 'printf "TOKEN=%s\n" "$INNER_TOKEN"'
    env:
      INNER_TOKEN: "{{ INNER_TOKEN }}"
```

```
$ F025_REPRO_TOKEN="hunter2-secret-value" mooncake apply -c /tmp/f025-repro.yml
▶ use the stashed secret
  | TOKEN=__MOONCAKE_SECRET_v1_DO_NOT_EDIT__:env:F025_REPRO_TOKEN
~ use the stashed secret
```

Two problems in one line of stdout:

1. **The secret is unresolved.** The literal sentinel reaches
   the subprocess instead of the env-var value.
2. **The redactor never learned the value.** If the user *had*
   typed the password as a template that did happen to resolve
   correctly, it would print as cleartext — there is no second
   line of defense.

Same repro with `F025_REPRO_TOKEN` unset crashes much later (the
shell sees an empty `$INNER_TOKEN`) instead of failing fast at
resolve time with "env var FOO not set" the way F019's redactor
arm should.

## Why it's a bug

Spec-23 §3 promises that `!secret env:FOO` resolves "just before
the handler sees them." For the vars action, the "handler" is
effectively the planner, and the resolver step is missing.

This is the **dominant use case for `!secret` in real configs**:
people park secrets in a top-level `vars:` action so subsequent
steps can reference them via templates. Every such config today
leaks the sentinel.

## Fix sketch

Two options; preference for B.

**Option A — call Resolve inside `expandVars`.**

```go
func (p *Planner) expandVars(step config.Step, ctx *ExpansionContext) error {
    if step.Vars == nil {
        return fmt.Errorf("vars step has nil Vars field")
    }
    // spec-23 §3: resolve markers BEFORE merging into the user-scope
    // template context. Plan mode is a no-op (markers stay).
    if !p.planMode {
        if err := resolver.Resolve(&step, p.redactor); err != nil {
            return fmt.Errorf("resolve vars secrets: %w", err)
        }
    }
    for k, v := range *step.Vars {
        // ... existing render loop ...
    }
    return nil
}
```

Pros: surgical, single call site, plumbing already exists for
F019.

Cons: the planner now needs to know about the redactor — minor
new dependency on `internal/security`. Survey existing planner
imports first.

**Option B — pre-resolve markers in the YAML reader, eagerly.**

Move secret resolution to the boundary between YAML decode and
planning (e.g., in `internal/config/reader.go` right after the
secret_tag rewrite). The resolver becomes a one-shot pass that
mutates the loaded `Config` in place; downstream code (planner,
executor) never sees markers.

Pros: removes the closed-kind-set problem entirely; F019 + F024
+ F025 all become moot because the markers are gone before
walkers run.

Cons: bigger change. Plan-mode redaction needs a different path
(today it re-extracts the `!secret <ref>` form from the marker;
post-fix it would need a separate "redaction pass" that runs
even in plan mode, OR the early pass must be plan-mode aware).

**Recommendation: A first (1-day fix, no spec change). Track B
as a separate refactor.**

## Regression test

`internal/plan/planner_test.go`:

```go
func TestExpandVars_ResolvesSecretsBeforeMerge(t *testing.T) {
    t.Setenv("F025_TEST_TOKEN", "leaked")
    step := config.Step{
        Vars: &map[string]interface{}{
            "INNER_TOKEN": security.SentinelPrefix + "env:F025_TEST_TOKEN",
        },
    }
    ctx := newPlanCtx()
    if err := planner.expandVars(step, ctx); err != nil {
        t.Fatal(err)
    }
    if got := ctx.Variables["INNER_TOKEN"]; got != "leaked" {
        t.Fatalf("INNER_TOKEN = %q, want %q", got, "leaked")
    }
}
```

Plus an end-to-end test in `cmd/cmd_test.go` (or wherever
`mooncake apply` is exercised) that runs the repro spec above
and asserts the stdout does NOT contain `__MOONCAKE_SECRET_v1_`.

## References

- `internal/plan/planner.go:325-329` — planner intercept point.
- `internal/plan/planner.go:599-620` — `expandVars` body.
- `internal/actions/vars/handler.go:110-113` — handler's own
  doc-comment foreshadowing this gap.
- `internal/executor/executor.go:1253` — the only Resolve call
  site today.
- F019 — fixed `resolver.Resolve` to handle map kinds; correct
  fix, wrong call site for the headline case.
- F024 — analogous "closed kind set" miss in `walkAndRender`,
  filed by another agent. Same root cause, different walker.
