package kernel

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/plan"
)

func TestPlanResourceIdentifier_FileShown(t *testing.T) {
	ins := plan.StepInspection{
		Diff: &actions.Diff{
			Resource: actions.ResourceRef{
				Kind:       actions.ResourceFile,
				Identifier: "/tmp/x.txt",
			},
		},
	}
	if got := planResourceIdentifier(ins); got != "/tmp/x.txt" {
		t.Errorf("got %q, want /tmp/x.txt", got)
	}
}

func TestPlanResourceIdentifier_NilDiff(t *testing.T) {
	// log/cmd/assert have no Differ implementation — Diff is nil.
	if got := planResourceIdentifier(plan.StepInspection{}); got != "" {
		t.Errorf("got %q, want empty (no Diff)", got)
	}
}

func TestPlanResourceIdentifier_EmptyIdentifier(t *testing.T) {
	ins := plan.StepInspection{
		Diff: &actions.Diff{
			Resource: actions.ResourceRef{Kind: actions.ResourceFile},
		},
	}
	if got := planResourceIdentifier(ins); got != "" {
		t.Errorf("got %q, want empty (no identifier)", got)
	}
}

func TestPlanResourceIdentifier_ShellSuppressed(t *testing.T) {
	// Shell-kind reason already echoes the command on the main line,
	// so adding "→ <cmd>" would be redundant.
	ins := plan.StepInspection{
		Diff: &actions.Diff{
			Resource: actions.ResourceRef{
				Kind:       actions.ResourceShell,
				Identifier: "echo hello",
			},
		},
	}
	if got := planResourceIdentifier(ins); got != "" {
		t.Errorf("got %q, want empty (shell suppressed)", got)
	}
}
