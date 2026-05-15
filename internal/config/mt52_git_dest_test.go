package config

import (
	"strings"
	"testing"
)

// TestMT52_GitConfig_DestAlias is a regression test for manual-test
// #52 (2026-05-15): chaining git.clone → git.config → git.checkout
// required renaming the path parameter twice in adjacent steps
// (`dest:` → `repo:` → `dest:`). Accept `dest:` on git.config as an
// alias for `repo:` so the param is consistent across the family.
func TestMT52_GitConfig_DestAlias(t *testing.T) {
	cases := []struct {
		name    string
		gc      GitConfig
		want    string
		wantErr string
	}{
		{"repo only", GitConfig{Repo: "/srv/r"}, "/srv/r", ""},
		{"dest only", GitConfig{Dest: "/srv/r"}, "/srv/r", ""},
		{"both same", GitConfig{Repo: "/srv/r", Dest: "/srv/r"}, "/srv/r", ""},
		{"both conflict", GitConfig{Repo: "/srv/a", Dest: "/srv/b"}, "", "both `repo:` and `dest:`"},
		{"both empty", GitConfig{}, "", ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got, err := c.gc.WorkingTree()
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("err = %v, want substring %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
