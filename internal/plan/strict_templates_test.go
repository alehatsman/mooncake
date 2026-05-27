package plan

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestCheckPlanStrict_FlagsUndefinedRoot(t *testing.T) {
	plan := &Plan{
		InitialVars: map[string]interface{}{"user_home": "/home/u"},
		Steps: []config.Step{
			{
				ID:   "step-0001",
				Name: "typo demo",
				FileWrite: &config.File{
					Path: "{{ user_hom }}/test.txt",
				},
			},
		},
	}
	refs := CheckPlanStrict(plan)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d: %+v", len(refs), refs)
	}
	if refs[0].Root != "user_hom" {
		t.Errorf("Root = %q, want user_hom", refs[0].Root)
	}
	if refs[0].StepName != "typo demo" {
		t.Errorf("StepName = %q, want typo demo", refs[0].StepName)
	}
}

func TestCheckPlanStrict_AcceptsKnownRoot(t *testing.T) {
	plan := &Plan{
		InitialVars: map[string]interface{}{"user_home": "/home/u"},
		Steps: []config.Step{
			{
				FileWrite: &config.File{
					Path: "{{ user_home }}/test.txt",
				},
			},
		},
	}
	if refs := CheckPlanStrict(plan); len(refs) != 0 {
		t.Errorf("expected no refs, got %+v", refs)
	}
}

func TestCheckPlanStrict_AcceptsItemAndEnv(t *testing.T) {
	plan := &Plan{
		Steps: []config.Step{
			{
				FileWrite: &config.File{
					Path:    "/tmp/{{ item }}",
					Content: "PATH={{ env.PATH }}",
				},
			},
		},
	}
	if refs := CheckPlanStrict(plan); len(refs) != 0 {
		t.Errorf("expected no refs for item/env, got %+v", refs)
	}
}

func TestCheckPlanStrict_AcceptsPriorRegister(t *testing.T) {
	plan := &Plan{
		Steps: []config.Step{
			{
				ID: "step-0001",
				Cmd: &config.CommandAction{
					Argv: []string{"echo", "hello"},
				},
				As: "greeting",
			},
			{
				ID: "step-0002",
				FileWrite: &config.File{
					Path: "/tmp/{{ greeting }}",
				},
			},
		},
	}
	if refs := CheckPlanStrict(plan); len(refs) != 0 {
		t.Errorf("expected no refs for prior register, got %+v", refs)
	}
}

func TestCheckPlanStrict_RejectsSelfRegister(t *testing.T) {
	// A step cannot reference its own `as:` register — that's a
	// forward reference to a value that doesn't exist yet.
	plan := &Plan{
		Steps: []config.Step{
			{
				FileWrite: &config.File{
					Path: "/tmp/{{ greeting }}",
				},
				As: "greeting",
			},
		},
	}
	refs := CheckPlanStrict(plan)
	if len(refs) != 1 || refs[0].Root != "greeting" {
		t.Errorf("expected self-register to be flagged, got %+v", refs)
	}
}

func TestCheckPlanStrict_RejectsForwardRegister(t *testing.T) {
	plan := &Plan{
		Steps: []config.Step{
			{
				ID: "step-0001",
				FileWrite: &config.File{
					Path: "/tmp/{{ greeting }}",
				},
			},
			{
				ID: "step-0002",
				Cmd: &config.CommandAction{
					Argv: []string{"echo", "hello"},
				},
				As: "greeting",
			},
		},
	}
	refs := CheckPlanStrict(plan)
	if len(refs) != 1 || refs[0].Root != "greeting" {
		t.Errorf("expected forward register to be flagged, got %+v", refs)
	}
}

func TestCheckPlanStrict_SkippedStepDoesNotFlag(t *testing.T) {
	// Tag-filtered or when-skipped steps don't reach the executor,
	// so flagging their undefined refs is noise.
	plan := &Plan{
		Steps: []config.Step{
			{
				FileWrite: &config.File{
					Path: "{{ undefined }}/x",
				},
				Skipped: true,
			},
		},
	}
	if refs := CheckPlanStrict(plan); len(refs) != 0 {
		t.Errorf("expected skipped step not to be flagged, got %+v", refs)
	}
}

func TestCheckPlanStrict_DedupesWithinStep(t *testing.T) {
	plan := &Plan{
		Steps: []config.Step{
			{
				FileWrite: &config.File{
					Path:    "{{ missing }}/a",
					Content: "{{ missing }}-content",
				},
			},
		},
	}
	refs := CheckPlanStrict(plan)
	if len(refs) != 1 {
		t.Errorf("expected single ref (dedup), got %d: %+v", len(refs), refs)
	}
}

func TestMissingRoots_IgnoresBuiltins(t *testing.T) {
	known := map[string]bool{}
	for k := range pongo2Builtins {
		known[k] = true
	}
	got := missingRoots("{{ true }} or {{ none }}", known)
	if len(got) != 0 {
		t.Errorf("builtins should not be flagged, got %v", got)
	}
}

func TestMissingRoots_OrderedFirstOccurrence(t *testing.T) {
	got := missingRoots("{{ a }} {{ b }} {{ a }} {{ c }}", map[string]bool{})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("got[%d] = %q, want %q", i, got[i], v)
		}
	}
}
