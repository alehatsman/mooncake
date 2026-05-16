# Bug — `failed_when` reports fabricated "exit code 1" on actually-succeeded commands

**Tracking:** [#21](https://github.com/alehatsman/mooncake/issues/21)
**Surfaced:** 2026-05-15 during the control-flow primitives test sweep
(try/catch/finally + continue_on_error + changed_when + failed_when).

## Repro

```yaml
- name: failed_when=true on clean exit 0
  shell:
    cmd: "echo body"
  failed_when: "true"
```

```
$ mooncake apply -c fw.yml
▶ failed_when=true on clean exit 0
command failed with exit code 1                # ← lies. Actual exit was 0.
✗ failed_when=true on clean exit 0
  command failed with exit code 1
RECAP  ok=0  changed=1  skipped=0  failed=1  1s  ✗ command failed with exit code 1
{"event":"step_error","ts":"...","step":"...","action":"shell","exit_code":-1,"stdout":"body\n"}
```

The event payload's `"stdout":"body\n"` confirms the command did
run and exit 0 (otherwise we'd see no stdout, and `exit_code:-1`
suggests no underlying child exit was captured at all).

Three layers wrong:

1. **The user-visible text "command failed with exit code 1"** is
   fabricated. The command did not exit 1. There's no exit code 1
   anywhere in this run.
2. **The event's `exit_code: -1`** is the "no exit code captured"
   sentinel, not the actual 0. So the runtime *knows* the underlying
   exit was clean — it just doesn't surface that to the operator.
3. **The recap text** repeats the wrong number.

## Why this matters

`failed_when:` is the mechanism for "the command technically ran
fine but I want to fail the step based on its output / context". A
typical pattern:

```yaml
- shell: { cmd: kubectl get pod foo -o json }
  register: r
  failed_when: "r.stdout | parse_json | .status.phase != 'Running'"
```

The whole point is: command's exit code is irrelevant; the
*condition* is what determined failure. The error message should
*say that* — "step marked failed by failed_when expression", or
better, include the expression that evaluated true. Instead it
fabricates "exit code 1" which sends operators chasing a non-
existent shell failure.

## Root cause — hypothesis

Without diving into the executor, the failure-rendering path
probably:

1. Notices `step.failed_when` evaluated true
2. Synthesises a generic "failed" outcome for the step
3. Reuses the shell-action error renderer, which expects an
   `exit_code` field
4. Defaults the missing exit_code to 1

The fix is in the rendering: when the step's failure source is the
`failed_when` expression (not a real subprocess failure), the
message should reflect that. Something like:

```
✗ failed_when=true on clean exit 0
  marked failed by failed_when (expression: "true", actual exit: 0)
```

This also gives the operator the actionable info they need:
"oh, my predicate was wrong" vs "oh, my command failed."

## Fix outline

In the executor's step-result rendering path:

```go
if step.HasFailedWhenOverride() && step.UnderlyingExitWasClean() {
    return formatFailedWhenFailure(step) // new message path
}
return formatShellExitFailure(step)      // existing path
```

`formatFailedWhenFailure` would render the expression source +
the *real* underlying exit code (0, or whatever it was — the run
captured 0 successfully into stdout).

## Test gap

`internal/executor/*_test.go` probably has a test that asserts
"failed_when:true causes step to fail" but doesn't assert anything
about the *content* of the failure message. Adding:

```go
require.Equal(t, "marked failed by failed_when (expr=...)", step.ErrorMessage)
require.Equal(t, 0, step.UnderlyingExitCode)
```

…would catch this.

## Workaround

None at the surface level — the step really did fail, the message
is just misleading. Operators chasing this end up adding logging
themselves to find that their underlying command actually exited 0.

## Related observation

`continue_on_error` rendering also looks confused for the same
step — the step gets marked with both `~` (changed) and `✗`
(failed) markers in different lines of the same output:

```
▶ continue_on_error step (exit 7)
  [WARNING] Ignoring error (ignore_errors: true): command failed with exit code 7
✗ continue_on_error step (exit 7)
  command failed with exit code 7
~ continue_on_error step (exit 7)
```

The `✗` line and the `~` line refer to the same step — once
because it failed, once because continue_on_error said "ignore
it." Probably want one consolidated render with a clear
"failed-but-ignored" marker. Filing separately if it doesn't
fold into MT-48's retry-honors-failed_when:false work.
