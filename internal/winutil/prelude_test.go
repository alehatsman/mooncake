package winutil

import (
	"strings"
	"testing"
)

// Issue #13 regression tests: the UTF-8 output prelude must prefix
// every PowerShell payload so cmdlets like ConvertTo-Json /
// Export-ScheduledTask don't round-trip non-ASCII data through the
// OEM codepage and corrupt it.

func TestUTF8OutputPrelude_ContainsBothEncodingLines(t *testing.T) {
	// Both lines are necessary — see prelude.go's doc comment.
	if !strings.Contains(UTF8OutputPrelude, "[Console]::OutputEncoding") {
		t.Error("prelude missing Console::OutputEncoding assignment")
	}
	if !strings.Contains(UTF8OutputPrelude, "$OutputEncoding") {
		t.Error("prelude missing $OutputEncoding assignment")
	}
	if !strings.Contains(UTF8OutputPrelude, "UTF8") {
		t.Errorf("prelude missing UTF8 selector: %q", UTF8OutputPrelude)
	}
}

func TestUTF8OutputPrelude_EndsWithNewline(t *testing.T) {
	// Without a trailing newline the prelude bleeds into the caller's
	// first statement on the same line. PowerShell does treat `;` and
	// newlines as statement separators, so a missing newline is a
	// readability/debuggability issue more than a correctness one,
	// but we want to assert the contract.
	if !strings.HasSuffix(UTF8OutputPrelude, "\n") {
		t.Errorf("prelude must end with newline so caller's script starts cleanly: %q", UTF8OutputPrelude)
	}
}

func TestWithUTF8Output_PrependsPrelude(t *testing.T) {
	script := "Get-Process | ConvertTo-Json"
	got := WithUTF8Output(script)
	if !strings.HasPrefix(got, UTF8OutputPrelude) {
		t.Errorf("WithUTF8Output should prepend prelude, got:\n%s", got)
	}
	if !strings.HasSuffix(got, script) {
		t.Errorf("WithUTF8Output should preserve original script verbatim at the tail, got:\n%s", got)
	}
}

func TestWithUTF8Output_IdempotentForEmptyScript(t *testing.T) {
	// Empty inner script is uncommon but legal — the prelude still
	// emits valid PowerShell on its own.
	got := WithUTF8Output("")
	if got != UTF8OutputPrelude {
		t.Errorf("empty input should yield bare prelude: %q", got)
	}
}
