package apply

// The runlog projection for failed steps. A record that names the step but
// not the reason answers half the question a post-mortem asks, so error text
// and exit code ride along — bounded, because package-manager failures fold
// the tail of the command's output into their error.

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestBuildStepEntries_FailedStepCarriesReason(t *testing.T) {
	failed := executor.NewResult()
	failed.Failed = true
	failed.Rc = 1
	failed.Error = "command failed with exit code 1"

	okResult := executor.NewResult()

	entries := buildStepEntries([]executor.StepRecord{
		{Step: config.Step{Name: "start ollama", Shell: &config.ShellAction{Cmd: "brew services start ollama"}}, Result: okResult},
		{Step: config.Step{Name: "pull model", Shell: &config.ShellAction{Cmd: "ollama pull x"}}, Result: failed},
	})

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Result != "ok" {
		t.Errorf("first entry Result = %q, want ok", entries[0].Result)
	}
	if entries[0].Error != "" || entries[0].ExitCode != 0 {
		t.Errorf("a successful step must carry no error/exit_code, got %q/%d", entries[0].Error, entries[0].ExitCode)
	}
	if entries[1].Result != "failed" {
		t.Errorf("second entry Result = %q, want failed", entries[1].Result)
	}
	if entries[1].Error != "command failed with exit code 1" {
		t.Errorf("Error = %q, want the step's reason", entries[1].Error)
	}
	if entries[1].ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", entries[1].ExitCode)
	}
}

// TestTruncateErr_KeepsTail: the diagnosis is at the end of a package
// manager's output, the command that produced it is at the start — and the
// step record already names the command.
func TestTruncateErr_KeepsTail(t *testing.T) {
	if got := truncateErr("short"); got != "short" {
		t.Errorf("a short message must pass through unchanged, got %q", got)
	}

	long := strings.Repeat("x", maxRunlogErrBytes) + "Error: No available formula"
	got := truncateErr(long)
	if len(got) > maxRunlogErrBytes+len("…") {
		t.Errorf("truncated length = %d, want <= %d", len(got), maxRunlogErrBytes+len("…"))
	}
	if !strings.HasSuffix(got, "Error: No available formula") {
		t.Errorf("tail lost; got suffix %q", got[max(0, len(got)-40):])
	}
	if !strings.HasPrefix(got, "…") {
		t.Error("truncation must be visible in the record")
	}
}
