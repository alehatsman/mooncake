package executor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/plan"
)

// unmatchedTagsError returns a user-facing message when a non-empty
// --tags filter doesn't intersect any tagged step in the plan, and
// "" when the filter is unused (empty) or at least one tagged step
// matched. Empty return means "no issue; proceed".
//
// The detection is intentionally narrow: untagged steps still count
// as runnable (they're scaffolding the user always wants — see
// utils.MatchesTags). The bug we want to catch is "user typed
// `--tags deplly`, intended `deploy`, and nothing from their tagged
// set will run." When that happens we surface the available tags and
// the closest-match suggestion so they can recover without re-reading
// the playbook.
func unmatchedTagsError(filterTags []string, p *plan.Plan) string {
	if len(filterTags) == 0 || p == nil {
		return ""
	}
	available := collectStepTags(p.Steps)
	if len(available) == 0 {
		// No step in the plan has any tags — filter is meaningless but
		// the user might have copied the command from elsewhere. Keep
		// quiet here (untagged "always-on" semantics still apply).
		return ""
	}
	availableSet := map[string]struct{}{}
	for _, t := range available {
		availableSet[t] = struct{}{}
	}
	for _, f := range filterTags {
		if _, ok := availableSet[f]; ok {
			return ""
		}
	}
	// No filter tag matched any step's tag. Build a helpful message.
	sort.Strings(available)
	var msg strings.Builder
	msg.WriteString("no steps matched tags: ")
	msg.WriteString(strings.Join(filterTags, ", "))
	for _, f := range filterTags {
		if best := closestTag(f, available); best != "" {
			fmt.Fprintf(&msg, ". Did you mean: %s?", best)
			break
		}
	}
	fmt.Fprintf(&msg, " (available: %s)", strings.Join(available, ", "))
	return msg.String()
}

// collectStepTags walks the plan steps (and any nested children that
// carry their own steps — transactions, on_change, try branches) and
// returns every distinct tag found.
func collectStepTags(steps []config.Step) []string {
	seen := map[string]struct{}{}
	var walk func(ss []config.Step)
	walk = func(ss []config.Step) {
		for _, s := range ss {
			for _, t := range s.Tags {
				seen[t] = struct{}{}
			}
			if len(s.OnChange) > 0 {
				walk(s.OnChange)
			}
			if len(s.Transaction) > 0 {
				walk(s.Transaction)
			}
			if len(s.OnRollback) > 0 {
				walk(s.OnRollback)
			}
			if len(s.Try) > 0 {
				walk(s.Try)
			}
		}
	}
	walk(steps)
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	return out
}

// closestTag returns the lexically-nearest available tag to `needle`
// using a cheap edit-distance metric, or "" if no candidate is within
// a reasonable threshold. Threshold scales with the longer string so
// long/short mismatches don't dominate.
func closestTag(needle string, candidates []string) string {
	best := ""
	bestDist := 1 << 30
	for _, c := range candidates {
		d := levenshtein(needle, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	// Suggest only when the edit distance is small relative to length —
	// avoids "deploy" suggesting "x" on plans where every tag is short.
	maxLen := len(needle)
	if len(best) > maxLen {
		maxLen = len(best)
	}
	if maxLen > 0 && bestDist*3 <= maxLen*2 {
		return best
	}
	return ""
}

// levenshtein returns the edit distance between a and b. Standard DP;
// used only at human typo scale (≤ a few dozen tags × a few characters).
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
