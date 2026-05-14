// Package recommend produces fact-aware preset suggestions for the
// `mooncake presets recommend` command. The catalogue is embedded in
// the binary today; once a preset marketplace exists this can grow
// into a fetched-and-cached resource.
package recommend

import "github.com/alehatsman/mooncake/internal/facts"

// Profile is the minimal subset of facts the recommendation engine
// keys off. Kept as a struct so we can extend (e.g. desktop_env,
// container runtime) without breaking the catalogue's matching code.
type Profile struct {
	OS             string // "linux" | "darwin" | "windows"
	PackageManager string // "apt" | "pacman" | "dnf" | "brew" | ""
}

// ProfileFrom collapses a Facts struct into the recommendation Profile.
// Defensive against nil so callers can pass facts.Collect() unchecked.
func ProfileFrom(f *facts.Facts) Profile {
	if f == nil {
		return Profile{}
	}
	return Profile{
		OS:             f.OS,
		PackageManager: f.PackageManager,
	}
}

// Recommend returns a deduplicated list of preset names suitable for the
// profile, filtered against the user's locally-known preset set. Order
// is curated (base entries first, then profile-specific) — NOT
// alphabetical, NOT facts-randomised. Stable across runs so users see
// the same top suggestions.
//
// known is the set of preset names the user can currently install
// (typically built from internal/presets.PresetSearchPaths). Pass nil
// to skip the filter — useful for tests or when discovery hasn't run.
//
// limit caps the result length. limit <= 0 returns all candidates.
func Recommend(p Profile, known map[string]bool, limit int) []string {
	seen := map[string]bool{}
	out := []string{}

	add := func(name string) {
		if seen[name] {
			return
		}
		if known != nil && !known[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}

	// Base entries first: useful on every profile.
	for _, e := range catalogue {
		if e.base {
			add(e.name)
		}
	}
	// Profile-specific entries.
	for _, e := range catalogue {
		if e.base {
			continue
		}
		if matches(e.profile, p) {
			add(e.name)
		}
	}

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// matches returns true when entry's profile constraints are satisfied by
// the user's profile. An empty constraint field means "any value".
func matches(entry, user Profile) bool {
	if entry.OS != "" && entry.OS != user.OS {
		return false
	}
	if entry.PackageManager != "" && entry.PackageManager != user.PackageManager {
		return false
	}
	return true
}
