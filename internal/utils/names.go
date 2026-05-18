package utils

// MatchesNames reports whether a step should run under the given name
// filter. Spec-50 `--step-filter name=<step>` propagates these names from
// the controller to the planner.
//
// Semantics (chosen to match operator intent of "target this specific
// step"):
//
//   - Empty filter            → every step runs (no name filter active).
//   - Step has no name        → step is skipped (name filter is exact
//     match; an unnamed step can't be named).
//   - Step has a name         → must match the filter exactly.
//
// Note this differs from MatchesTags: untagged steps run on a tag filter
// (untagged ≡ infrastructure), but unnamed steps are dropped on a name
// filter. Reason: the explicit ask is "run the step *called* X"; an
// unnamed step is by definition not the one the operator asked for.
//
// Match is exact — no globs, no prefix matching. The spec defers pattern
// matching to a future `name~=<glob>` operator if a real use case appears.
func MatchesNames(stepName string, filterNames []string) bool {
	if len(filterNames) == 0 {
		return true
	}
	if stepName == "" {
		return false
	}
	for _, f := range filterNames {
		if stepName == f {
			return true
		}
	}
	return false
}
