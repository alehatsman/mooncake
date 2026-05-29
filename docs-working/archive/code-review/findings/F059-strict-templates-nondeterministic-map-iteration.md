---
id: F059
title: CheckPlanStrict claims deterministic output but iterates maps (step.Env, props, nested action-struct maps) in random order — unresolved-template diagnostics reorder run-to-run, churning --format json output and any golden test
severity: smell
package: internal/plan
file: internal/plan/strict_templates.go
lines: 65-107 (CheckPlanStrict docstring + loop), 112-168 (collectStepRefs / scan), 160-162 (step.Env range), 236-243 (walkStringLeaves reflect.Map), 255-270 (walkPropsLeaves map case)
status: done
fixed: 2026-05-30 (fix-f059, worktree-f059) — made the three map-iteration sites deterministic at the source rather than post-hoc sorting the output. `step.Env` (collectStepRefs) and `props` (walkPropsLeaves) now iterate via a new `sortedStringKeys` generic helper; `walkStringLeaves`'s `reflect.Map` case sorts `MapKeys()` by `.String()` before recursing. Source-sorting (vs the tail-sort the original write-up proposed) fixes BOTH symptoms in one consistent pass: the output slice order AND the field attributed to a deduplicated root (lowest sorted key now deterministically wins the per-step dedup) — a tail-sort would have left the latter wobbling since the dedup keeps only one entry per root. Two regression tests in `strict_templates_test.go`: `TestCheckPlanStrict_DeterministicMapOrder` (50 iterations, asserts sorted-key emission order) and `TestCheckPlanStrict_DeterministicFieldAttribution` (50 iterations, asserts the lowest sorted key wins dedup). `go test ./...` green (133 pkgs), gofmt/vet clean. The adjacent duplicated-`rootVarRe`/`pongo2Builtins` drift smell was left as-is (latent, no divergence today — the dedup-the-copies cleanup is a separate one-liner not in scope here).
discovered: 2026-05-29 — cold-read of internal/plan gaps (planner.go end-to-end + strict_templates.go, the latter new on 2026-05-28 and never formally reviewed). PICKUP item 1 ("continue the code-review cold-read").
related: F048 (non-strict YAML), F057 (pongo2 renderer global-state), the F002/F013 "pinned constants drift" theme (the duplicated rootVarRe/pongo2Builtins below is that shape). Adjacent observation, not a separate finding: convertToSlice (planner.go:1396) is another test-only //nolint:unused helper — same family as the executor.go dead-helper theme tracked in code-review/TODO.md "Cross-cutting themes".
---

## What

`CheckPlanStrict` (`strict_templates.go:68`) documents itself as
returning **deterministic** output:

```go
// CheckPlanStrict scans the expanded plan for unresolved root
// identifiers. Returns a deterministic list (steps in order, refs
// per step in field declaration order, deduplicated by root).
```

Steps are indeed visited in order, and the fixed scalar fields (`when`,
`changed_when`, `failed_when`, `cwd`, `as_user`, `timeout`, `name`,
`shell.*`, `creates`, `unless*`) are scanned in declaration order. But
three of the scanned surfaces are **Go maps**, iterated in randomized
order:

1. `step.Env` — `for key, value := range step.Env` (`strict_templates.go:160`).
2. Action-struct maps reached by reflection — `walkStringLeaves`'s
   `reflect.Map` case (`strict_templates.go:236-243`) uses
   `rv.MapRange()`.
3. `step.Props` (spec-67) — `walkPropsLeaves`'s
   `map[string]interface{}` case (`strict_templates.go:255-270`).

Two observable consequences:

- **Slice order is nondeterministic.** Given a step with
  `env: {A: "{{ foo }}", B: "{{ bar }}"}` where both `foo` and `bar`
  are undefined, the returned `[]UnresolvedRef` is `[foo, bar]` on one
  run and `[bar, foo]` on the next. `plan.UnresolvedTemplates` is
  emitted **verbatim** into `--format json`
  (`cmd/kernel/validate.go:117-122`, `cmd/kernel/plan.go:170-174`), so
  the JSON payload reorders between identical runs.

- **Field attribution is nondeterministic.** The per-step dedup set
  (`seen`, `strict_templates.go:98`) persists across *all* fields of a
  step and reports each root once, first-scanned-field-wins. When a
  root first appears in a map-valued field, *which* field is recorded
  in `UnresolvedRef.Field` varies run-to-run (`env.A` vs `env.B`).

## Why this is a smell, not a correctness bug

The *set* of flagged roots is correct and stable — only the order and
the field-label attribution wobble. The human-facing text path is
mostly insulated: `FormatUnresolvedTemplates`
(`cmd/kernel/unresolved.go:43-51`) sorts its group headers by
`(file, line, col)`. So the operator-facing `validate` / `plan` stderr
is stable *across steps*, but the per-line `field → {{ root }}` lines
within one origin still print in slice order (random for map fields),
and the structured `--format json` output is fully exposed.

Impact lands in two places:

- **Flaky golden/snapshot tests.** Any test that asserts on
  `CheckPlanStrict`'s slice or on the JSON `unresolved_templates` array
  is order-dependent and will flake. (No such test exists *today* —
  which is partly why this slipped in — but the docstring's explicit
  "deterministic" promise invites one.)
- **Diff churn for machine consumers.** A fleet/agent flow that diffs
  successive `plan --format json` outputs to detect drift sees phantom
  changes in the `unresolved_templates` ordering.

## Reproduction

```yaml
steps:
  - name: typo demo
    shell: "echo hi"
    env:
      ALPHA: "{{ undef_one }}"
      BRAVO: "{{ undef_two }}"
```

```
mooncake plan --config demo.yml --format json 2>/dev/null \
  | jq -c '.unresolved_templates | map(.root)'
```

Run it a handful of times: the array flips between
`["undef_one","undef_two"]` and `["undef_two","undef_one"]` as Go
randomizes the `step.Env` range. (Map-iteration randomization is
per-process, so you may need several runs to see both orders.)

## Proposed fix

One localized change closes all three sources: sort the assembled
`out` slice before returning, with a stable key that matches the
documented contract. At the tail of `CheckPlanStrict`
(`strict_templates.go:106`):

```go
// The per-step visit order is already deterministic for scalar
// fields, but step.Env / props / nested action-struct maps iterate
// in random order. Sort by (step position, field, root) so the
// documented "deterministic list" contract actually holds and
// --format json output is byte-stable across runs.
sort.SliceStable(out, func(i, j int) bool {
    if out[i].StepID != out[j].StepID {
        return stepPos[out[i].StepID] < stepPos[out[j].StepID]
    }
    if out[i].Field != out[j].Field {
        return out[i].Field < out[j].Field
    }
    return out[i].Root < out[j].Root
})
```

(`stepPos` = a `map[stepID]int` of visit order, or carry the step
index on `UnresolvedRef` and sort on it — either keeps cross-step
order intact while making intra-step map-field order stable.)

The alternative — sort each map's keys at every iteration site — is
three edits instead of one and still leaves the cross-field dedup
race; the single tail-sort is the cleaner seam. The field-attribution
wobble (which map entry "wins" the dedup) is then resolved by the
`Field` tiebreaker in the sort.

## Adjacent observation (not a separate finding): duplicated parse constants

`strict_templates.go:50,54` define `rootVarRe` and `pongo2Builtins` as
**byte-identical copies** of the authoritative versions in
`internal/template/renderer.go:20,22`. The strict-checker's own
comment acknowledges the coupling ("Matches the renderer's pattern
(renderer.go) so the two stay in sync"), but nothing enforces it. The
renderer is the source of truth — it's what `RenderPreserving`
actually uses to decide which `{{ root }}` placeholders to keep. If
the renderer's regex or builtin set changes (e.g. a new builtin, or a
trim-marker tweak), the strict checker silently drifts: roots the
renderer preserves but the checker no longer recognizes become false
positives (and vice-versa become false negatives). Same shape as the
F002/F013 "pinned constant drifts within a sprint" theme. Cheap fix:
export the two from `internal/template` and have `plan` import them, so
there is one definition. Logged here rather than as its own finding
because it is latent (no divergence today) and the export is a
one-liner whenever someone touches either site.

## Verified clean in the same pass (no finding)

- `expandInclude` (`planner.go:476`) — cycle detection via
  `p.seenFiles` with `defer delete`, and the include-stack frame popped
  via `defer`; both clean up on every error path.
- `readRunConfig` (`planner.go:329`) re-parses on every call (no
  cache), so the in-place `includedConfig.Steps[i].Tags = mergeTags(...)`
  mutation (`planner.go:534`) cannot corrupt a shared slice across
  sibling includes of the same file.
- `expandWithItems` scalar form (`planner.go:739-741`) routes through
  `template.ResolveList` rather than the pongo2 renderer, with an
  explicit comment about avoiding pongo2's
  `<[]interface {} Value>` slice-stringification — the for_each
  breakage class is handled.
- `RenderPreserving` (`renderer.go:370`) correctly falls back to plain
  `Render` whenever the template contains a `{%` control tag, so
  `{% for x in ... %}{{ x }}{% endfor %}` collapses at plan time rather
  than leaking the loop var `x` to `CheckPlanStrict` as a false
  positive. (This was the first thing I suspected; it's defended.)
