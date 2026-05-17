# Wave 2 — Agent prompt (R1.1a)

**Date:** 2026-05-15
**Source:** [`2026-05-15-refactoring-plan.md`](./2026-05-15-refactoring-plan.md) §3 (R1.1a)
**Wave:** 2 of 4 — pattern proof
**Gate:** all of Wave 1 (R0.1, R0.2, R0.3, R0.4, R3.1) is in master before firing.
**Unlocks:** Wave 3 (R1.1b + R2.1a) can run after R1.1a lands.

Wave 2 is a single PR. The mission is **mechanical**: lift the body
of `cmd.run` into a new `internal/apply.Runner` and leave `cmd.run`
as a thin shim. **No** typed `KernelResult` return shape, **no**
`Reverse()` method — those are R1.1b's job in Wave 3.

The PR proves the cmd-extraction pattern that R2.1a will copy at
larger scale. Review it carefully; if the shape lands clean here,
the next two waves follow the same outline.

## How to use

1. Confirm all of Wave 1 is in master: `git log --oneline | grep -E "R0\.|R3\."`
   should show 5 R-items merged. If R0.3 / R0.4 haven't merged yet, hold.
2. Copy the fenced block below into a fresh agent session.
3. Agent creates its own worktree, claims, does the work, runs tests,
   commits to its worktree branch, reports back. Does **not** merge or push.
4. Review the PR carefully — the shape of `internal/apply.Runner` is what
   R2.1a will mirror for fleet, and what R1.1b will type-up for `KernelResult`.
   Worth a slow read.

---

## Prompt — R1.1a: pure extraction of `apply.Runner` from `cmd.run`

````
You are executing R1.1a of the Mooncake refactoring plan: pure
mechanical extraction of cmd.run's body into a new
internal/apply.Runner package. cmd.run becomes a thin shim. No new
typed return shape (that's R1.1b in Wave 3).

## Read first (load context)

1. /home/aleh/projects/mooncake/docs-working/arch-report/2026-05-15-refactoring-plan.md
   — find the R1.1a section under §3. Read the full Motivation /
   Files touched / DONE when / Risk sections.
2. /home/aleh/projects/mooncake/docs-working/vision/kernel.md
   — Mooncake is a typed mutation kernel. cmd.run is the kernel's
   Apply() entry point trapped inside a CLI handler. This PR frees
   it. The MCP server, agent loop, and future SDK consumers all need
   this entry point — today they re-implement parts.
3. /home/aleh/projects/mooncake/CLAUDE.md — project conventions
   (claims protocol, worktree workflow, no commit/push without
   explicit ask).

## Mission

cmd.run (cmd/mooncake.go:236, gocyclo 33) does: config resolution,
vars layering, tag filtering, plan building, plan-or-execute
selection, artifacts writing, run audit. None of that is "CLI parse
→ call → render."

This PR is the CODE-MOVE-ONLY half. Lift the body out into
internal/apply.Runner. Leave the return shape flat (matching today's
behavior — typically `error` and whatever recap shape cmd.run
already produces). cmd.run becomes a shim that builds an
apply.Config from CLI flags and calls apply.NewRunner(cfg).Run().

DO NOT introduce a typed KernelResult, DO NOT add a Reverse()
method. Those are R1.1b (Wave 3). Bundling them here makes review
hard and is the explicit reason for the wave split.

## Files

New:
- internal/apply/runner.go — `Runner` struct + `Run()` method.
  Body is lifted verbatim from cmd.run; only adjustments needed are
  the ones the move requires (Config struct fields instead of CLI
  flags, etc.).
- internal/apply/config.go — `Config` struct holding the fields
  cmd.run accepts as CLI flags. Field names should mirror the flag
  names (Config string, VarsFiles []string, Tags string, etc.).
- internal/apply/runner_test.go — at least one apply-path
  integration test that exercises the new entry point on an
  examples/ plan.
- internal/apply/doc.go — package doc that cites
  docs-working/vision/kernel.md as the rationale for the
  package's existence (this is the kernel's Apply() entry point).

Modified:
- cmd/mooncake.go — `run` collapses to ~50 LOC of flag parse →
  apply.Config construction → Runner.Run() call → recap render.
- cmd/cmd_test.go — relevant tests retargeted to the new API
  where cheaper. Test set should otherwise be identical.

## Watch / risks

- (a) Hidden side effects in cmd.run (logging setup, env-var
  reading, defer cleanup). Diff stdout/stderr from a scripted
  apply on examples/ before/after; they must match.
- (b) Avoid pulling in fleet-apply needs. R1.1a covers local apply
  only; fleet-apply (R2.1a) gets its own orchestrator. Don't try
  to make apply.Config accommodate fleet-style options.
- (c) Circular imports. After the move, run `go list -e ./...`;
  any cycle fails the build immediately. internal/apply will likely
  import internal/executor + internal/plan; that's fine.

## Claims protocol (load-bearing — concurrent agents read this file)

Append to /home/aleh/.mooncake/claims.jsonl as JSONL lines.
Get UTC via `date -u +%FT%TZ`.

1. claimed (at start):
   {"ts":"<ISO>","task":"R1.1a","status":"claimed","worktree":"<path>","branch":"<branch>","note":"R1.1a pure extraction of cmd.run"}
2. in-progress (after baseline passes):
   same shape, status:"in-progress"
3. done (after commit):
   same shape, status:"done","sha":"<commit-sha>"
4. abandoned (if blocked):
   same shape, status:"abandoned","note":"<blocker>"

## Constraints (do not break)

- PURE relocation. Every test that passes before passes after.
- DO NOT introduce KernelResult, FleetKernelResult, or Reverse()
  methods.
- DO NOT merge to master. DO NOT push. Commit only to worktree
  branch.
- DO NOT use --no-verify. Hooks fire normally.
- If anything blocks, append abandoned claim and stop.

## Workflow

1. cd /home/aleh/projects/mooncake
2. git worktree add /home/aleh/projects/mooncake-r1.1a-apply -b worktree-r1.1a-apply
3. cd /home/aleh/projects/mooncake-r1.1a-apply
4. Append claimed line.
5. go test ./... baseline. Known pre-existing failure:
   internal/fleet/discovery/TestAggregate_* (mDNS flakes) — ignore.
   If anything else fails, STOP.
6. Append in-progress line.
7. Do the extraction:
   - mkdir -p internal/apply
   - Create internal/apply/{runner.go, config.go, doc.go, runner_test.go}
   - Move cmd.run body into Runner.Run()
   - Reduce cmd/mooncake.go's run function to flag-parse + Runner.Run() + render
8. go build ./... — must succeed
9. go test ./... — must pass (modulo mDNS)
10. go vet ./... — clean
11. Verify gocyclo: `gocyclo cmd/mooncake.go | grep -E "^[0-9]+ main run"`
    should show ≤ 10 (was 33).
12. Verify LOC reduction: `wc -l cmd/mooncake.go` should be ~300 lines
    smaller than before.
13. Scripted parity test:
    Pick examples/ plan A (e.g. examples/dotfiles).
    On master: `mooncake apply -c examples/A --dry-run 2>&1 > /tmp/before.txt`
    On your worktree (built): same → /tmp/after.txt
    `diff /tmp/before.txt /tmp/after.txt` should be empty (modulo timestamps).
14. git add -A && git commit -m:
    "refactor(R1.1a): extract apply.Runner from cmd.run

    Pure mechanical extraction: cmd.run's body moves into
    internal/apply.Runner; cmd.run becomes a flag-parse + Runner.Run() +
    recap-render shim. Return shape stays flat — typed KernelResult
    + Reverse() come in R1.1b (Wave 3).

    The kernel's Apply() entry point is now callable by any frontend
    (MCP server, agent loop, future SDK) without re-implementing
    config resolution, vars layering, tag filtering, plan build,
    artifacts writing, or run audit.

    No behavior change: every existing test passes; scripted apply
    on examples/ produces byte-identical output.

    See docs-working/arch-report/2026-05-15-refactoring-plan.md
    §3 (R1.1a) and docs-working/vision/kernel.md."
15. Append done line with commit sha.

## Report (final message — keep tight)

```
task:     R1.1a
status:   done | abandoned
worktree: <absolute path>
branch:   <branch name>
sha:      <commit sha>
gocyclo:  cmd.run = N  (was 33; target ≤ 10)
loc:      cmd/mooncake.go = N  (was ~1456; target ≥ 300 LOC drop)
tests:    PASS | FAIL <details>
parity:   examples/ scripted apply byte-identical | divergence at <line>
notes:    <anything surprising or worth a human eye>
```
````
