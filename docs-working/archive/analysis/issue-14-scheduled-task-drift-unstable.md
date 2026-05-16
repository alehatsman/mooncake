# Bug — `windows.scheduled_task` drift detection unstable across round-trip

**Tracking:** [#14](https://github.com/alehatsman/mooncake/issues/14)
**Surfaced:** 2026-05-15 during the spec-56 / spec-57 retest cycle.
Right after a clean `apply`, the next `plan` still reports
"would update task WSL2-SSH-Keepalive". Subsequent applies converge
to no observable system change, but the operator-facing diagnosis is
permanently wrong: drift is reported where none exists.

This is cosmetic (no system harm, idempotent in effect), but it
breaks the load-bearing invariant "plan after apply is empty",
which is what makes `check` mode useful as a CI gate.

---

## Root cause

`internal/winutil/scheduledtask.go:NormaliseTaskXML` only does two
things:

1. Strips whitespace-only lines.
2. Sorts the children of `<Settings>` alphabetically (because Task
   Scheduler reorders that block).

It does NOT account for what Task Scheduler injects at register-time:

- `<RegistrationInfo><Author>` (the username of the registering
  account)
- `<RegistrationInfo><Date>` (the registration timestamp)
- `<RegistrationInfo><URI>` may get rewritten to absolute form
- `<Principals><Principal id="Author">` gets canonicalised attribute
  ordering
- Defaulted `<Settings>` elements that we omit get added with their
  schema defaults (e.g. `<AllowHardTerminate>true</AllowHardTerminate>`)
- `<IdleSettings>` with empty subelements often appears
- `<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>` always appears
  even when omitted from input (Task Scheduler emits the default)
- Attribute value casing on some elements

When mooncake re-renders the desired XML and compares against the
live one, every one of these is "drift" and the action reports
"would update". Apply re-registers, the round-trip produces the same
canonicalised output, plan reports drift again. Stable loop.

---

## Fix

Two layers:

### 1. Stronger canonicalisation in `NormaliseTaskXML`

Strip the elements Task Scheduler will inject anyway before
comparing:

```go
var taskSchedulerInjected = map[string]struct{}{
    "<Author>":           {},
    "<Date>":             {},
    "<AllowHardTerminate>": {},
    "<IdleSettings>":     {},  // entire subtree if empty
    "<NetworkSettings>":  {},  // ditto
    "<WakeToRun>":        {},
    // ...
}
```

Walk both XML documents, strip these elements (and their closing
tags), sort `<Settings>` children, compare normalised strings.

### 2. Switch from string-compare to semantic compare

The XML-string approach was chosen for speed; semantic compare
(unmarshal both into a `Task` struct, compare with reflect.DeepEqual
after both sides go through `withDefaults()`) would be more robust
but requires writing a Task XML *parser* — currently we only have a
renderer. The parser doesn't have to handle every Task Scheduler
schema feature, just the ones our renderer emits + the ones the
scheduler injects.

Trade-off: option 1 is ~30 LOC, lands today; option 2 is ~200 LOC
plus a roundtrip test. Recommend option 1 as a stopgap; option 2
when we add more trigger types (the canonicalisation list will keep
growing under option 1).

---

## Diagnostic during the discovery

Comparing live vs rendered XML side-by-side would have caught this
immediately. A useful debug aid for spec-57's eventual hardening:
add a `WINDOWS_SCHEDULED_TASK_DEBUG_DIFF=1` env var that, when set,
dumps both XML documents (live + desired) to `%TEMP%` with the
suffix `.live.xml` / `.desired.xml` so the operator can `diff` them.

---

## Workaround

None needed operationally — the action is idempotent in effect, just
chatty. Plan output reading "would update task X" after every apply
should be interpreted as "live XML differs textually from rendered
desired XML; system state is fine."

---

## Related

The original spec-57 draft (`docs-working/specs/action-surface/spec-57-windows-firewall-and-scheduled-task-actions.md`,
§"Open questions" 2) explicitly flagged this:

> **XML diff fidelity.** Scheduled-task definitions roundtrip through
> the Task Scheduler service, which may reorder elements or add
> default attributes. A naive XML compare would over-report drift.
> Probably needs a canonicalization step (sort attrs, drop
> service-added defaults) — same problem spec-26's git-status parser
> solves.

The fix here is the deferred canonicalization step that §2 promised.
