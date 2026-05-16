# Code Review — Continuous Pass

**Start:** 2026-05-16
**Reviewer:** Claude (continuous-review loop)
**Method:** Iterative per-area review. Each iteration picks one
package or one function, reads it cold, writes findings to
`findings/F<NNN>-<slug>.md`, and commits.

This directory exists so **other agents can pick up findings and fix
them independently** without re-reading the whole codebase. Each
finding file is self-contained: file path, line numbers, severity,
suggested fix, and verification steps.

## Read this first

- [00-baseline.md](./00-baseline.md) — build / test / lint /
  arch-snapshot status at start of pass.
- [TODO.md](./TODO.md) — what's reviewed, what's queued, what's
  unblocked for fix work.
- [findings/](./findings/) — per-issue write-ups.

## How to use as a fixer agent

1. Open [TODO.md](./TODO.md) and pick the first finding in
   **"Unblocked — ready for fix"** with severity matching your
   priority filter.
2. Open `findings/F<NNN>-*.md`. The file tells you:
   - exact file path + line numbers
   - what's wrong
   - suggested fix (and its alternatives, if any)
   - how to verify (build / test / lint command)
3. Make the change in a new worktree (per CLAUDE.md rules).
4. Mark the finding's `status:` field to `done` and add a
   `resolved_by:` pointer (commit SHA or PR #).

## Severity legend

| Severity | Meaning |
|---|---|
| `bug` | Wrong behavior. Reproducible. Will surface to a user. |
| `risk` | Latent — wrong under conditions not currently exercised (cross-platform, race, error path). |
| `smell` | Working but obscures intent. Refactor target. |
| `readability` | Working & clear — but with a small expressivity win. |
| `perf` | Measurable performance cost on a hot path. |
| `doc` | Drift between code and CLAUDE.md/arch-report/specs. |

## Methodology / scope

- **Targets**: arch-soft-cap violators first, then handlers, then
  shared infrastructure, then cmd, then tests.
- **Out of scope**: anything explicitly closed in
  `docs-working/arch-report/2026-05-16-code-review.md` unless a new
  finding emerges.
- **Each finding cites file:line.** No "the foo handler is messy" —
  always concrete.
- **No fixes in this pass** — review only. Fixers run separately.

## Cross-cutting themes (running list)

Patterns observed across packages, ordered by leverage.

1. **Spec-16 migration incomplete in 24 handlers** (F011). The
   `Execute` / `DryRun` / `Run` triple still ships in every
   action package except `copy` and `file` (which arch-wins
   migrated). ~1,000 LOC of deletable legacy code; the
   `actions.Handler` interface can shrink once the migration
   finishes.

2. **Unbounded HTTP** (F012, F007, F014). 9 packages use
   `http.Get` / `http.DefaultClient` / `http.NewRequest` with
   no timeout, no context. Mooncake runs as a CI / provisioning
   agent; any hung HTTP call blocks an apply. `internal/httputil`
   helper + per-call migration is the consolidated fix.

3. **Cancellation invariant broken from the kernel boundary
   inward** (F016, F020). `apply.Runner.Run(ctx)` accepts a
   context but doesn't observe it; the executor doesn't either.
   `apply.Runner.installSignalHandler` does `os.Exit` on
   SIGTERM, hostile to agentd / MCP. End-state: SIGTERM hangs
   or aborts mid-run depending on caller.

4. **`sudo -S` shell-out reimplemented 6 ways** (F005, F004).
   Inconsistent guards mean become-unsupported produces 3
   different error shapes. `internal/security.BecomeRunner`
   would consolidate.

5. **Documentation drift around tracked numbers** (F002, F013,
   F021). Counts, action lists, lifecycle contracts pinned in
   doc-strings drift within days. Lean on `make budget-status`
   and on inline-derived counts; never pin a number in two
   places.

6. **Reflection-based walkers with closed kind sets** (F019).
   The secret resolver walks struct fields by kind; the closed
   set misses `*map[string]interface{}` (step.Vars). New shapes
   added in the future will silently pass through. A
   verification-walk at the end of `Resolve()` would catch
   missed markers loudly.

7. **Cleanup invariants in agentd worker not enforced** (F015,
   F016, F017). "Every exit path of executeRun must run the
   same cleanup" isn't symmetric across paths — F015 found one
   missed close. Defer-based cleanup pattern is the fix.

8. **Unbounded buffer / scanner sizes** (F018). `bufio.Scanner`
   default 64 KB max-line, `bytes.Buffer` with no cap.
   Subprocess-output capture path is the worst offender; audit
   `shell`, `assert`, `observe_logs`, `wait_command`.

9. **Stale `//nolint:gocyclo` directives** (F017 adjacent obs).
   Functions that were over the cap and got extracted no longer
   need the suppression; the directive stays.

## Summary of findings

| Severity | Count | IDs |
|---|---:|---|
| bug | 6 | F015, F017, F018, F019, *((F010 dead test, treated as smell here))* |
| risk | 6 | F001, F007, F012, F014, F016, F020 |
| smell | 7 | F003, F004, F005, F006, F009, F010, F011 |
| readability | 1 | F008 |
| doc | 4 | F002, F013, F021, (F009 partial) |

(Counts approximate — some findings span multiple severities.)

The two top-priority fixers should look at:

- **F019** (security defense-in-depth): `!secret` silently
  doesn't resolve in `step.Vars`.
- **F015** (real bug): worker chdir-error leaks hub →
  goroutine leak on SSE side.
- **F017** (real bug): `continue_on_error` emits both
  step.failed and step.completed; SSE consumers see the
  step flip-state.
- **F020** (risk): `apply.Runner` calls `os.Exit`, hostile
  to agentd / MCP.

Quick-win XS fixes: F002 (CLAUDE.md), F013 (config.Step doc),
F021 (Config.ExtraSubscribers doc), F010 (dead test).
