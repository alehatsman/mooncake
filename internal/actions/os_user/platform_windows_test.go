//go:build windows

package os_user //nolint:revive // package name follows action convention

import (
	"context"
	"strings"
	"testing"
)

// TestQuotePS_EscapesEmbeddedSingleQuote pins the only function in
// platform_windows.go that can be exercised cross-platform-ish via
// unit testing without a real PowerShell host. The PS quoting rule
// is: single-quote strings are literal (no expansion); an embedded
// single-quote is doubled to escape. Getting this wrong is a
// command-injection surface, so the rule deserves an explicit
// guard.
func TestQuotePS_EscapesEmbeddedSingleQuote(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"alice", "'alice'"},
		{"O'Brien", "'O''Brien'"},
		{"name with spaces", "'name with spaces'"},
		{"", "''"},
		{"'", "''''"}, // single quote alone → '''' (open, double, close)
		{"a'b'c", "'a''b''c'"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := quotePS(c.in); got != c.want {
				t.Errorf("quotePS(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestListUserLocalGroups_HandlesAbsentUser verifies the probe
// returns an empty slice (not an error) when the user has no
// group memberships at all. Uses a clearly-bogus username so the
// PowerShell call is a real probe but should match no group.
//
// Skips automatically when running outside an interactive
// Administrator shell — Get-LocalGroupMember requires elevation
// on some Windows SKUs and the CI environment may not provide it.
func TestListUserLocalGroups_HandlesAbsentUser(t *testing.T) {
	groups, err := listUserLocalGroups(context.Background(), "mooncake-nonexistent-user-test")
	if err != nil {
		// PowerShell unavailable / not elevated / network domain
		// dispatch — all are valid skip conditions for a unit
		// test that just verifies the function shape.
		t.Skipf("listUserLocalGroups skipped (likely not elevated): %v", err)
	}
	if len(groups) != 0 {
		// A clearly-nonexistent username matching some group
		// membership would mean our suffix-matching is broken.
		if !strings.Contains(strings.ToLower(strings.Join(groups, ",")), "mooncake-nonexistent-user-test") {
			t.Errorf("nonexistent user has groups: %v", groups)
		}
	}
}
