---
id: F002
title: CLAUDE.md "Today's known violations" list is stale vs `make budget-status`
severity: doc
package: (repo root)
file: CLAUDE.md
lines: 70–86
status: open
---

## What

`CLAUDE.md` §"Today's known violations" (line 70+) lists:

```
- internal/actions/file — 2,044 LOC
- internal/actions/tool — 1,676 LOC
- internal/actions/service — 1,607 LOC
- internal/actions/package — 1,216 LOC (within 20% of cap)
- explain.DisplayFacts — gocyclo 53
- copy.(*Handler).Execute — gocyclo 41
- executor.ExecuteStep — gocyclo 37
- internal/config.Step — 36 universal fields (within 20% of cap)
```

Actual `make budget-status` output as of 2026-05-16 commit `2e9714d`:

```
1. Handler LOC vs cap 1500
   ⚠ file                                      1349 LOC (within 20%)
   ⚠ package                                   1216 LOC (within 20%)
   ✗ service                                   1607 LOC (over)
   ✗ tool                                      1676 LOC (over)

2. Non-test functions vs gocyclo cap 35
   ✗ gocyclo=44  explain.DisplayFacts          internal/explain/explain.go:55:1

3. internal/config.Step universal-field count vs cap 40
   ⚠ 36 universal fields (within 20%)
```

Three items in the CLAUDE.md list are wrong:

1. **`internal/actions/file` — 2,044 LOC** → now 1,349 (the
   arch-wins `Execute`/`DryRun` deletion in `b23fd4f` shed 590
   lines from `handler.go` + 116 from tests).
2. **`copy.(*Handler).Execute` — gocyclo 41** → function deleted in
   `27e9ade`. Not on the budget report anymore.
3. **`executor.ExecuteStep` — gocyclo 37** → reduced below 35 by
   `dispatchPlanMode` + `postExecuteSuccess` extractions in
   `27e9ade`. Not on the budget report anymore.

The line above the list (line 72) already says **"Run
`make budget-status` for the current source-of-truth list"** — but
then the list right below pins specific numbers that have already
drifted in the same commit range that wrote that line.

## Why it's `doc` not `risk`

CLAUDE.md is read by future agents to set context. A stale list of
violations causes two failure modes:

1. **False urgency** — an agent thinks `copy.Execute` is gocyclo 41
   and goes hunting for it, finds nothing, gets confused.
2. **Missed real signals** — `gocyclo 44 explain.DisplayFacts` is
   the *only* real gocyclo violation today and went up from 53→44
   only because helpers were extracted. The CLAUDE.md list still
   shows 53; readers may not realize the budget tightened.

## Suggested fix

Either:

**Option A — delete the inline list entirely** and lean on
`make budget-status`. Removes drift risk by removing the duplicated
truth.

**Option B — regenerate the list from `make budget-status`** as
part of the same step that updates the date stamp on the section.
Aligns the list and the date.

Option A is the safer long-run move: every prior incarnation of
"here's the list of violations" has drifted out of sync within
24h. The README has the running list, the budget script has the
live list, the CLAUDE.md inline list is just a third copy.

## Verification

Run `make budget-status` after the change. The output should match
whatever CLAUDE.md references.

## References

- Side-findings merge commit `6f4cde0`: last update to the soft-cap
  section (touched the field count + dropped `fleetApplyAction` and
  `os_systemd.computePlan`).
- `b23fd4f`: deleted the `Execute`/`DryRun` paths from `copy` and
  `file`, which is what dropped the `file` LOC and removed
  `copy.Execute` from the gocyclo list.
- `27e9ade`: executor `ExecuteStep` extractions that dropped its
  gocyclo number.
