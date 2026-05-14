package utils

import "testing"

func TestMatchesNames(t *testing.T) {
	tests := []struct {
		name   string
		step   string
		filter []string
		want   bool
	}{
		{"empty filter matches anything", "install nvim", nil, true},
		{"empty filter matches unnamed step", "", nil, true},
		{"exact hit", "install nvim", []string{"install nvim"}, true},
		{"exact miss", "install zsh", []string{"install nvim"}, false},
		{"hit in second slot", "install zsh", []string{"install nvim", "install zsh"}, true},
		{"unnamed step is dropped on any name filter", "", []string{"install nvim"}, false},
		{"case sensitive: capitalized step name vs lowercase filter", "INSTALL", []string{"install"}, false},
		{"no substring matching: prefix doesn't hit", "install", []string{"install nvim"}, false},
		{"no substring matching: suffix doesn't hit either", "install nvim plugin", []string{"install nvim"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesNames(tt.step, tt.filter)
			if got != tt.want {
				t.Errorf("MatchesNames(%q, %v) = %v, want %v",
					tt.step, tt.filter, got, tt.want)
			}
		})
	}
}
