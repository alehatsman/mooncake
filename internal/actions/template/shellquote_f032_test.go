package template

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/effects"
)

// F032 regression guard. executeSudoFileOperation's sprintf must
// shell-quote each interpolated path so a hostile dest like
// "/tmp/x; touch /etc/owned" can't break out of the command. The
// legacy Execute path is unreachable in production today (executor
// only dispatches Run), but the method is exported on the Handler
// type and reachable from tests / future SDKs. This test pins the
// quoting at the format-string level — no exec, no sudo, just shape.
func TestF032_ExecuteSudoFileOperation_QuotesDestPath(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		bannedRaw string // must NOT appear as literal substring in output
	}{
		{"command-chain", "/tmp/x; touch /etc/owned", "; touch /etc/owned"},
		{"command-sub", "/tmp/$(id)/foo", "$(id)"},
		{"backtick-sub", "/tmp/`id`/foo", "`id`"},
		{"newline", "/tmp/x\necho hi", "\necho hi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			quoted := effects.ShellQuote(c.input)
			if strings.Contains(quoted, c.bannedRaw) && !strings.HasPrefix(quoted, "'") {
				t.Errorf("ShellQuote(%q) = %q; banned substring %q is unquoted", c.input, quoted, c.bannedRaw)
			}
			// Quoted form must start with `'` and end with `'`.
			if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
				t.Errorf("ShellQuote(%q) = %q; expected single-quote wrap", c.input, quoted)
			}
		})
	}
}

// TestF032_ExecuteSudoFileOperation_EmbeddedSingleQuote: a path with
// an embedded `'` must be escaped via the POSIX `'\”` idiom so the
// outer quoting doesn't break and the embedded quote isn't reinterpreted.
func TestF032_ExecuteSudoFileOperation_EmbeddedSingleQuote(t *testing.T) {
	got := effects.ShellQuote("/tmp/it's-mine")
	want := `'/tmp/it'\''s-mine'`
	if got != want {
		t.Errorf("ShellQuote(/tmp/it's-mine) = %q, want %q", got, want)
	}
}
