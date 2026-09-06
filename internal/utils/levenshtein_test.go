package utils

import "testing"

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"file.wrte", "file.write", 1}, // transposition-ish: one insert
		{"wehn", "when", 2},            // transposition = 2 substitutions
		{"kitten", "sitting", 3},       // the textbook case
		{"tags", "tagz", 1},            // substitution
		{"become", "as_user", 6},       // nowhere near — rename table's job
		{"héllo", "hello", 1},          // rune-aware, not byte-aware
	}
	for _, tc := range cases {
		if got := Levenshtein(tc.a, tc.b); got != tc.want {
			t.Errorf("Levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// Distance is symmetric.
func TestLevenshtein_Symmetric(t *testing.T) {
	pairs := [][2]string{{"file.write", "file.wrte"}, {"kitten", "sitting"}, {"", "x"}}
	for _, p := range pairs {
		if a, b := Levenshtein(p[0], p[1]), Levenshtein(p[1], p[0]); a != b {
			t.Errorf("asymmetric for %q/%q: %d vs %d", p[0], p[1], a, b)
		}
	}
}
