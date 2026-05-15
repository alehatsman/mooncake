package utils

import "testing"

func TestMatchesTags(t *testing.T) {
	tests := []struct {
		name       string
		stepTags   []string
		filterTags []string
		want       bool
	}{
		{"no filter, no step tags", nil, nil, true},
		{"no filter, step has tags", []string{"a"}, nil, true},
		{"filter set, step untagged → runs", nil, []string{"wsl"}, true},
		{"filter set, step untagged (empty slice) → runs", []string{}, []string{"wsl"}, true},
		{"filter matches one of step's tags", []string{"wsl", "core"}, []string{"wsl"}, true},
		{"filter matches none of step's tags", []string{"darwin"}, []string{"wsl"}, false},
		{"multi-tag filter, any match wins", []string{"wsl"}, []string{"darwin", "wsl"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesTags(tt.stepTags, tt.filterTags)
			if got != tt.want {
				t.Errorf("MatchesTags(%v, %v) = %v, want %v",
					tt.stepTags, tt.filterTags, got, tt.want)
			}
		})
	}
}

// MT-58: MatchesSkipTags returns true when a step's tags overlap
// skipTags (and the step should therefore be excluded).
func TestMatchesSkipTags(t *testing.T) {
	tests := []struct {
		name     string
		stepTags []string
		skipTags []string
		want     bool
	}{
		{"no skip, no step tags", nil, nil, false},
		{"no skip, step has tags", []string{"a"}, nil, false},
		{"skip set, step untagged → not excluded", nil, []string{"slow"}, false},
		{"skip set, step untagged (empty slice) → not excluded", []string{}, []string{"slow"}, false},
		{"skip matches one of step's tags → excluded", []string{"slow", "core"}, []string{"slow"}, true},
		{"skip matches none of step's tags → kept", []string{"fast"}, []string{"slow"}, false},
		{"multi-skip, any match excludes", []string{"slow"}, []string{"foo", "slow"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesSkipTags(tt.stepTags, tt.skipTags)
			if got != tt.want {
				t.Errorf("MatchesSkipTags(%v, %v) = %v, want %v",
					tt.stepTags, tt.skipTags, got, tt.want)
			}
		})
	}
}
