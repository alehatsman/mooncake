# Architecture reports

Periodic structural reviews of the codebase. One report per snapshot
in time. Newer reports do not invalidate older ones — they reflect
how the tree looked on the date in the filename.

Each report is **grounded in `docs-working/ARCH_SNAPSHOT.md`** (regenerate
via `scripts/arch-snapshot.sh` before writing a new one) plus a direct
read of the load-bearing packages. Reports are cross-reference, not
authority.

## Reports

| Date | File | Headline |
|---|---|---|
| 2026-05-16 (evening) | [`2026-05-16-refactor-plan-complete.md`](./2026-05-16-refactor-plan-complete.md) | **Refactor plan 100% complete.** 9/9 R-items in master. `fleetApplyAction` dropped off gocyclo > 15 (was 49). `internal/fleet` imports `internal/apply` — first non-cmd consumer of the kernel surface. Kernel-surface checkpoint met. Next: MCP → internal/apply, spec-66 typed plan diffs, R0.1-followup. |
| 2026-05-16 (morning) | [`2026-05-16-arch-report.md`](./2026-05-16-arch-report.md) | Wave 1 + R1.1a landed: 5 new packages materialize the kernel boundary `vision/kernel.md` described. `internal/control` lands at instability 0.00, validating the kernel-sub-system claim. R1.1b + R2.1a in flight; R2.1b gated. ~70% of refactor plan complete by R-item count. |
| 2026-05-15 | [`2026-05-15-arch-report.md`](./2026-05-15-arch-report.md) | Kernel healthy; pressure at edges (`cmd/`, `executor` accretion, `config.go` monotonic growth, handler tail). 4 mechanical extractions recommended. |

## Plans derived from reports

| Date | File | Status |
|---|---|---|
| 2026-05-15 | [`2026-05-15-refactoring-plan.md`](./2026-05-15-refactoring-plan.md) | **Complete.** 9 R-items shipped across 4 waves. See `2026-05-16-refactor-plan-complete.md` for the final accounting. |

## When to write a new one

- After a structural change (package split / merge, new top-level dir,
  ABI evolution, etc.).
- After a significant LOC delta (>10% kernel growth) since the last
  report.
- After a stream graduation (e.g. drafted → specced → shipped) that
  reshapes the dependency graph.
- Otherwise: quarterly.

## How to write one

1. Run `scripts/arch-snapshot.sh` to regenerate metrics.
2. Read the top 5 packages by LOC directly. Don't trust prior reports
   to describe current code.
3. Open with a one-paragraph executive summary that someone unfamiliar
   with the codebase could act on.
4. Use evidence — package metrics, gocyclo numbers, LOC, import counts
   — not impressions.
5. Order recommendations by leverage. State blast radius and risk for
   each.
6. List what the report intentionally does *not* cover, so reviewers
   know when to ask for a follow-up pass.
