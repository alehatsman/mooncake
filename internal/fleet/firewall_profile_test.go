package fleet

import "testing"

func TestPublicNetworkUncovered(t *testing.T) {
	cases := []struct {
		name string
		cats []string
		want bool
	}{
		{"only private", []string{"Private"}, false},
		{"only domain", []string{"DomainAuthenticated"}, false},
		{"public present", []string{"Private", "Public"}, true},
		{"public lowercase", []string{"public"}, true},
		{"public with spaces", []string{" Public "}, true},
		{"empty", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := publicNetworkUncovered(c.cats); got != c.want {
				t.Errorf("publicNetworkUncovered(%v) = %v, want %v", c.cats, got, c.want)
			}
		})
	}
}
