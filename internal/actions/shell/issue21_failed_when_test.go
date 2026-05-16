package shell

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestFailedWhen_CleanExit_DoesNotFabricateRc guards issue #21. When
// failed_when fires on a command that exited 0, the error message must
// say so explicitly — pre-fix it lied "command failed with exit code 1"
// and result.Rc was bumped from 0 to 1.
func TestFailedWhen_CleanExit_DoesNotFabricateRc(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	step := &config.Step{
		Shell:      &config.ShellAction{Cmd: "echo body"},
		FailedWhen: "true",
	}
	res, err := h.Run(ctx, step)
	if err == nil {
		t.Fatal("expected an error when failed_when=true")
	}
	if strings.Contains(err.Error(), "exit code 1") {
		t.Errorf("error message must not lie about exit code; got %q", err)
	}
	if !strings.Contains(err.Error(), "failed_when") {
		t.Errorf("error should mention failed_when as the failure source; got %q", err)
	}
	if r, ok := res.(*executor.Result); ok {
		if r.Rc != 0 {
			t.Errorf("result.Rc must reflect actual exit code (0), not fabricated 1; got %d", r.Rc)
		}
	}
}

// TestFailedWhen_RealNonzero_KeepsExitCode: when the underlying command
// exited nonzero, the error message should still cite that real exit
// code (issue #21 should NOT regress the legitimate-failure path).
func TestFailedWhen_RealNonzero_KeepsExitCode(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	step := &config.Step{
		Shell:      &config.ShellAction{Cmd: "exit 2"},
		FailedWhen: "true",
	}
	_, err := h.Run(ctx, step)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "exit code 2") {
		t.Errorf("error should cite the real exit code (2); got %q", err)
	}
}
