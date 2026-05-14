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
