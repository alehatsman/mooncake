//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"errors"
	"strings"
	"testing"
)

func TestTailOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		n      int
		want   string
	}{
		{name: "empty", output: "", n: 5, want: ""},
		{name: "whitespace only", output: "  \n  \n", n: 5, want: ""},
		{name: "fewer than n", output: "a\nb", n: 5, want: "a\nb"},
		{name: "trims surrounding whitespace", output: "\n\na\nb\n\n", n: 5, want: "a\nb"},
		{name: "keeps only last n", output: "a\nb\nc\nd", n: 2, want: "c\nd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tailOutput([]byte(tt.output), tt.n); got != tt.want {
				t.Fatalf("tailOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPkgCmdError(t *testing.T) {
	execErr := errors.New("exit status 1")

	// With output, the tail (the actual cause) is folded into the message.
	err := pkgCmdError("failed to install packages [packer]", execErr,
		[]byte("==> Searching...\nError: No available formula with the name \"packer\"."))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "failed to install packages [packer]") {
		t.Errorf("missing prefix in %q", msg)
	}
	if !strings.Contains(msg, "No available formula") {
		t.Errorf("output tail not surfaced in %q", msg)
	}
	// Wrapping is preserved so callers can still errors.Is/As the cause.
	if !errors.Is(err, execErr) {
		t.Errorf("execErr not wrapped: %v", err)
	}

	// Without output, it degrades to the plain wrapped error (no trailing newline).
	bare := pkgCmdError("failed to upgrade packages", execErr, nil)
	if got := bare.Error(); got != "failed to upgrade packages: exit status 1" {
		t.Errorf("bare error = %q", got)
	}
	if strings.Contains(bare.Error(), "\n") {
		t.Errorf("bare error should not contain a newline: %q", bare.Error())
	}
}
