package utils

// MatchesTags reports whether stepTags contains at least one tag from filterTags.
// If filterTags is empty, every step matches (no filter active).
// If stepTags is empty but filterTags is not, the step does not match.
func MatchesTags(stepTags, filterTags []string) bool {
	if len(filterTags) == 0 {
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
