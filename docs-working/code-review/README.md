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

Updated as patterns emerge.

- _(pending — first iteration in progress)_
