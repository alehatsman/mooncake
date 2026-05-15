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
| 2026-05-15 | [`2026-05-15-arch-report.md`](./2026-05-15-arch-report.md) | Kernel healthy; pressure at edges (`cmd/`, `executor` accretion, `config.go` monotonic growth, handler tail). 4 mechanical extractions recommended. |

## Plans derived from reports

| Date | File | Status |
|---|---|---|
| 2026-05-15 | [`2026-05-15-refactoring-plan.md`](./2026-05-15-refactoring-plan.md) | Ready for execution. 7 R-items across 4 phases; ~3,500 LOC of mechanical relocations. |

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
