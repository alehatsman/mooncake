# Spec 37: Step Output Capture — Collision & Plan-Mode Policy

**Status:** Draft
**Epic:** [Read & Report](../../epics/epic-read-and-report.md) — deliverable D2
**Effort:** XS (1 day)
**Value:** Framework. Defines two missing policies on the existing
`as:` capture keyword so the Read & Report epic can land cleanly on top.

**Design principles:** [action-design-principles.md](../../action-design-principles.md)

---

## Problem

The `as:` field on `Step` binds the step's primary result to a named
variable in the run scope:

```yaml
- repo.search:
    pattern: TODO
    path: ./internal
  as: todos

- log:
    msg: "found {{ todos.matches | length }} TODOs"
```

The mechanism works. Two policies around it are undefined.

### Problem 1 — collision handling

Two steps with the same `as:` name silently overwrite each other.
Writes happen at three sites — `executor.go:139-140`, `:430-434`,
`:978-979` — all unconditional:

```go
if step.As != "" {
    ec.Scope.Results[step.As] = result.ToRegisteredResult()
}
```

A typo'd duplicate produces wrong behavior with no signal. The
dominant *legitimate* duplicate-write pattern — a `for_each` loop
binding each iteration to the same name — also produces no signal,
which is fine for that case but means there's no way to distinguish
the two.

### Problem 2 — plan-mode policy

The current code writes captured results into `Scope.Results`
regardless of mode. For mutation actions this is wrong: a plan run
must not change the vars context that subsequent steps see.

For read-only actions the *opposite* is true — capturing a read
result during plan mode makes plan output more informative (next
step's `when: foo.found` can branch correctly in the plan preview).

There's no mechanism today for an action to declare which side of
that line it's on, so the executor has no basis on which to gate
the write.

The Read & Report epic's R1 spec (`read.json` / `read.yaml`) needs
this gate to exist before it ships; without it, R1 has to either
hand-roll a workaround inside its handler or leak side effects
into plan-mode vars.

---

## Goals

- **G1** Collision policy: last-write-wins, with a warning emitted
  to the run log on overwrite. `for_each` iterations (which
  legitimately rewrite the same name) are exempt from the warning.
- **G2** Plan-mode policy: `as:` does not populate `Scope.Results`
  in plan mode by default. Actions may opt in via a new capability
  flag on `ActionMetadata`, letting future read-only actions make
  their results available to downstream `when:` clauses during plan.
- **G3** Keep the existing `as:` keyword unchanged. No rename, no
  alias, no migration. `as:` is already modern syntax (Python/TS/
  Rust/SQL `… as …`); this spec only adds policies *around* it.

**Out of scope:**

- Renaming `as:` to anything else. Discussed and rejected — the
  current keyword is fine; the framework-internal names
  (`Scope.Results`, `ToRegisteredResult`) carry some Ansible-era
  vocabulary that this spec does not rename.
- Cross-step output graph / DAG analysis. Collision handling stays
  per-write; we do not pre-validate uniqueness across the plan.
- A general "step header" struct refactor (extracting
  `as`/`tags`/`when`/etc. into a sub-struct). Tempting; not here.
- Run-log capture of registered values. Separate concern; would
  land as its own spec if demand appears.

---

## Design

Two small mechanisms.

### Mechanism 1 — Collision warning

At each of the three executor write sites, wrap the assignment:

```go
name := step.As
if name == "" {
    return // unchanged path
}
if prev, exists := ec.Scope.Results[name]; exists && !sameForEach(step, prev) {
    ec.Svc.Logger.Warnf("output capture: name %q overwritten by step %s (previous value discarded)",
        name, step.ID)
}
ec.Scope.Results[name] = result.ToRegisteredResult()
```

`sameForEach(step, prev)` returns true when the prior writer was an
earlier iteration of the *same* `for_each` parent step. The
implementation needs the prior write to carry its source step ID;
either we widen `Scope.Results` to a small struct
(`{value, writtenBy stepID}`), or we track it side-table-keyed by
name. Pick the smaller diff during implementation — likely the side
table, to leave the value shape untouched for template consumers.

For the inaugural implementation, "same `for_each` parent" means
`step.ParentID != "" && step.ParentID == prevStep.ParentID`. Verify
during implementation that `ParentID` is populated post-expansion.

### Mechanism 2 — `CaptureInPlan` capability

`internal/actions/handler.go` — extend `ActionMetadata` with one
new field:

```go
type ActionMetadata struct {
    Name               string
    Description        string
    Category           Category
    SupportsDryRun     bool
    SupportsBecome     bool
    EmitsEvents        []string
    Version            string
    SupportedPlatforms []string
    RequiresSudo       bool
    ImplementsCheck    bool

    // CaptureInPlan declares that this action's result is safe to
    // bind into Scope.Results during plan mode. Reserved for
    // side-effect-free / observation-only actions whose result is
    // informative (e.g. read.json, read.yaml — to be added in R1).
    // Default false: mutation actions must not affect vars during plan.
    CaptureInPlan bool
}
```

The dispatcher (`executor.go:977-979` and the equivalent in the
other two write sites) gates the bind on mode + capability:

```go
canCapture := mode == actions.ModeApply || handler.Metadata().CaptureInPlan
if canCapture && name != "" {
    // (collision warning per Mechanism 1, then:)
    ec.Scope.Results[name] = result.ToRegisteredResult()
}
```

No built-in action sets `CaptureInPlan: true` in this spec. R1 will
flip it on for `read.json` and `read.yaml`. Shipping the capability
here means R1 doesn't need to touch the framework.

---

## Key files

| File | Change |
|---|---|
| `internal/actions/handler.go` | Add `CaptureInPlan bool` to `ActionMetadata`. |
| `internal/executor/executor.go` | Wrap the three write sites (lines 139-140, 430-434, 978-979) with the collision check + mode/capability gate. Add the side-table tracking the writer step ID per registered name. |
| `internal/executor/scope.go` | If we keep `Scope.Results` value-shape unchanged, no edit. If we widen, update the type. (Side-table is the recommended path — no public-shape churn.) |
| `internal/executor/executor_test.go` | Add tests per the matrix below. |
| `internal/config/config.go` | **No change.** `as:` already exists and parses. |
| `internal/config/schema.json` | **No change.** |
| `mooncake.d.ts` | **No change.** |
| Examples / presets | **No change.** Existing `as:` usage is untouched. |

This is a framework-only spec: zero new YAML surface, zero new
fields on `Step`, zero schema regen. The only public API delta is
the one new bool on `ActionMetadata`.

---

## Validation

No new YAML validation. `as:` already validates as "non-empty
string if present"; this spec adds nothing.

Implementation note: the collision warning fires at write time,
not at parse time. Catching duplicates at parse time would require
walking the step tree, accounting for `for_each` expansions, and
predicting which writes happen — not worth the complexity vs. a
runtime warning that fires exactly when the overwrite happens.

---

## Plan vs apply behavior

| Mode | `as:` set | Action `CaptureInPlan: false` | Action `CaptureInPlan: true` |
|---|---|---|---|
| Apply | "" | no-op | no-op |
| Apply | "foo" | bind `foo` in `Scope.Results` | bind `foo` in `Scope.Results` |
| Plan | "" | no-op | no-op |
| Plan | "foo" | **skip bind** | bind `foo` in `Scope.Results` |

The dry-run debug line in `executor/dryrun.go:53-54`
(`"Would register result as: %s"`) stays as a log hint even when
the bind is skipped — the user wrote `as: foo` and deserves to see
it acknowledged in plan output. (Update the log line wording in
passing: `"Would capture result as: %s"`.)

---

## Tasks

1. Add `CaptureInPlan bool` to `ActionMetadata` in
   `internal/actions/handler.go`. Run `go vet`, `make build` —
   default-false should be invisible to existing handlers.
2. Add the writer-step side table to `internal/executor/scope.go`
   (or scope-adjacent). Public API of `Scope.Results` unchanged.
3. Wrap the three executor write sites with the collision check +
   mode/capability gate. Factor the common logic into one helper
   (`func captureResult(ec, step, result)`) so the three call sites
   become single lines.
4. Update the dry-run log line wording.
5. Tests per the matrix below.
6. `make ci` clean.

---

## Tests

| Scenario | Layer |
|---|---|
| Two distinct steps with the same `as:` name → second emits warning, value overwrites | executor integration |
| `for_each` loop binding the same `as:` per iteration → no warning | executor integration |
| Mixed: `for_each` writes name `x`, then a non-loop step also writes name `x` → warning fires on the non-loop step | executor integration |
| Plan mode, action with default `CaptureInPlan: false` → `Scope.Results[name]` not written; next step's `when: foo.found` cannot see the value | plan integration |
| Plan mode, fixture action with `CaptureInPlan: true` → `Scope.Results[name]` written; next step's `when:` sees the value | plan integration |
| Apply mode, action with default `CaptureInPlan: false` → unchanged behavior (regression check) | e2e |
| Failed-step synthetic `{failed: true, rc: 1}` write at line 430-434 still happens in apply mode | executor integration |
| Failed-step write in plan mode follows the same `CaptureInPlan` gate as success | executor integration |
| Dry-run log line says "Would capture result as: …" | dryrun unit |

---

## Acceptance criteria

1. `make ci` clean.
2. Every existing config using `as:` continues to work unchanged
   in apply mode (regression suite: `examples/**/*.yml`,
   `presets/**/*.yml`).
3. No built-in action ships with `CaptureInPlan: true` in this
   spec. (R1 enables it for `read.json`/`read.yaml`.)
4. Collision warning fires exactly once when two distinct steps
   share an `as:` name; does not fire for `for_each` loop iterations.
5. No `schema.json` / `mooncake.d.ts` diff (this is a
   framework-only spec).

---

## Open questions

1. **Side-table vs widened `Scope.Results` value type.** Side table
   (`map[string]string` of name → writer step ID) is the smaller
   diff and leaves the template consumer surface untouched.
   Confirm during implementation that a side table is reachable
   from all three write sites without awkward plumbing.
2. **`for_each` parent ID availability.** Verify that the planner
   populates `Step.ParentID` (or equivalent field) on iteration-
   expanded steps. If not, the loop-exemption check needs an
   alternate signal — most likely a flag set during expansion.
   Check during implementation.
3. **Failed-step collision warning.** When a step fails and the
   `executor.go:430-434` path writes a synthetic
   `{failed: true, rc: 1}` over a previously-successful
   loop-iteration value, should the warning fire? Probably yes:
   the failure is a distinct write origin from the loop's
   successful iterations, and silently overwriting a useful value
   with a failure stub is exactly the kind of thing the user
   should see. Document this in the warning's docstring.
4. **Internal vocabulary churn.** `Scope.Results`, `ToRegisteredResult`,
   the `register: %s` log line — all carry vintage vocabulary. Not
   renamed in this spec to keep scope tight. If renaming happens
   later, do it as a single mechanical pass — don't dribble it in.

---

## Risk notes

- **Three hot-path write sites.** Adds one map lookup (collision
  check) and one bool branch (capability gate) per registered
  write. Both are O(1). No measurable cost.
- **Side-table consistency.** The writer-step side table must be
  written in lockstep with `Scope.Results` and copied on the same
  scope-clone events. If a scope copy misses the side table,
  collision detection fails closed (warning never fires). Cover
  with tests that exercise scope-cloning paths.
- **Behavior change on plan-mode bind.** Configs that currently
  rely on plan-mode `as:` bindings being visible to downstream
  `when:` clauses *will silently break* when this lands, because
  no built-in action sets `CaptureInPlan: true`. The
  agent-efficiency epic explicitly added drift detection on top
  of `mooncake plan`; verify no preset relies on a `when:`
  expression evaluating a captured result during plan. Grep
  `examples/` and `presets/` for `when:` clauses referencing
  `as:`-bound names before merge.
