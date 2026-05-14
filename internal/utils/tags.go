package utils

// MatchesTags reports whether a step should run under the given filter.
//
// Semantics (modern, not Ansible):
//
//   - Empty filter        → every step runs (no filter active).
//   - Step has no tags    → step runs (untagged ≡ infrastructure / always-on).
//   - Step has tags       → at least one tag must match the filter.
//
// Tagging is opt-IN to selective filtering: adding tags to a step gates it
// behind --tags / --step-filter. Steps with no tags are treated as
// scaffolding the user always wants when they invoke a filtered run, so
// `--step-filter tag=wsl` on a plan that mixes tagged + untagged steps
// runs every wsl-tagged step plus everything untagged.
func MatchesTags(stepTags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}
	if len(stepTags) == 0 {
		return true
	}
	for _, filterTag := range filterTags {
		for _, stepTag := range stepTags {
			if stepTag == filterTag {
				return true
			}
		}
	}
	return false
}
