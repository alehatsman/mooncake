# Bug — `continue_on_error: true` on a `try` compound step doesn't compose

**Tracking:** [#23](https://github.com/alehatsman/mooncake/issues/23)
**Surfaced:** 2026-05-15 during tick-4 of the autonomous test loop —
control-flow primitives investigation.

## Repro

```yaml
- name: try-with-continue
  continue_on_error: true     # ← intended: tolerate the compound's failure
  try:
    - name: fails
      shell: { cmd: "exit 5" }
  catch:
    - name: caught
      log: { msg: "in catch" }

- name: after try
  log: { msg: "POST-TRY RAN" }
```

```
$ mooncake apply -c try-continue.yml
▶ fails
✗ fails    command failed with exit code 5
▶ caught
✓ caught
RECAP  ok=1  changed=0  skipped=0  failed=1  1s  ✗ command failed with exit code 5
```

`after try` is never executed. The run halts after the compound step
is marked failed (per spec-23 §2: "outputs.success is true iff try
ran to completion without entering catch"). The user's
`continue_on_error: true` on the compound has no effect.

## Why this looks like a bug

`continue_on_error` is a universal Step field (see
`internal/config/config.go:Step.ContinueOnError`). Operators
reasonably expect "even if this step fails, keep going." With try
already handling failure inline (the catch ran, the cleanup did
its job), the operator's signal is: "yes, I want the run to
continue past this." Discarding `continue_on_error` on the
compound is surprising — it makes the field's contract feel like
"applies to action steps only".

If the docs explicitly stated "continue_on_error applies to leaf
action steps only; not to try/catch/transaction compounds," this
would be a documentation issue rather than a behavior bug. But
the field doc on the Step struct doesn't say that:

```go
ContinueOnError bool `yaml:"continue_on_error" json:"continue_on_error,omitempty"`
// no doc comment narrowing the field's applicability
```

## Workarounds

Both work, neither is intuitive:

### A. continue_on_error on the inner failing step

```yaml
- try:
    - name: fails-but-ignored
      shell: { cmd: "exit 5" }
      continue_on_error: true     # ← preempts catch entirely
    - name: still-in-try
      log: { msg: "after coe-step" }
  catch:
    - log: { msg: "wouldn't run" }
- log: { msg: "POST-TRY RAN" }  # ← runs because the try block's
                                #    failure was consumed
```

Side effect: `catch` never fires. The error is just ignored. If
the operator wanted cleanup (`catch:`) AND tolerance, this doesn't
work.

### B. Wrap the try in continue_on_error via a second compound

No such mechanism exists today. The compound's continue_on_error
field is silently ignored.

## Why catch+tolerate matters

The natural pattern is "run cleanup if the deploy fails, but don't
let the deploy failure stop the rest of the playbook":

```yaml
- name: deploy-app
  try:
    - pkg.install: { name: app }
    - file.template: { src: cfg.j2, dest: /etc/app/cfg }
    - os.service: { name: app, state: restarted }
  catch:
    - log: "deploy failed; rolling back"
    - shell: ./rollback.sh
  continue_on_error: true   # ← can't tolerate without this composing

- name: tell-monitoring
  shell: ./mark-degraded.sh
```

Without the compose, the operator has to either inline
continue_on_error on every step inside try (cumbersome and
breaks catch), or split deploy+rollback into a custom error-
handling preset.

## Fix

Honor `Step.ContinueOnError` in the compound-step executors
(`internal/executor/try.go`, `internal/executor/transaction.go`).
When the compound completes with failure AND the compound has
`ContinueOnError: true`:

- Convert the step's outcome to "failed-but-tolerated" (the
  existing convention used by leaf actions).
- Set `outputs.success: false` per spec-23 (don't paper over the
  truth).
- Don't halt the run.

Same logic likely applies to `transaction:` blocks — a transaction
that rolled back cleanly but `continue_on_error: true` is set
should also tolerate the failure.

## Test gap

`internal/executor/try_test.go` / `transaction_test.go` likely
have tests for the "try fails, run halts" case but not the
"try fails, continue_on_error set, run continues" case.

A two-step test would catch the regression instantly:

```go
plan := buildPlan(`
- try: [...failing step...]
  catch: [...recovery...]
  continue_on_error: true
- log: { msg: "tail" }
`)
result := executor.Run(plan)
assert.Equal(t, "tail step executed", result.LastStepName)
```
