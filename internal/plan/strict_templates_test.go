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

// TestCheckPlanStrict_DeterministicMapOrder pins F059: env values are
// scanned in sorted-key order, so the emitted refs are stable across
// runs regardless of Go's randomized map iteration. Without the fix
// the slice order flips run-to-run.
func TestCheckPlanStrict_DeterministicMapOrder(t *testing.T) {
	newPlan := func() *Plan {
		return &Plan{
			Steps: []config.Step{
				{
					ID:    "step-0001",
					Name:  "env demo",
					Shell: &config.ShellAction{Cmd: "echo hi"},
					Env: map[string]string{
						"ALPHA":   "{{ undef_b }}",
						"BRAVO":   "{{ undef_a }}",
						"CHARLIE": "{{ undef_c }}",
					},
				},
			},
		}
	}

	// Sorted by env key (ALPHA, BRAVO, CHARLIE), NOT by root name.
	wantFields := []string{"env.ALPHA", "env.BRAVO", "env.CHARLIE"}
	wantRoots := []string{"undef_b", "undef_a", "undef_c"}

	// Run several times: Go randomizes map iteration per range, so a
	// non-deterministic implementation would eventually disagree.
	for iter := 0; iter < 50; iter++ {
		refs := CheckPlanStrict(newPlan())
		if len(refs) != len(wantFields) {
			t.Fatalf("iter %d: got %d refs, want %d: %+v", iter, len(refs), len(wantFields), refs)
		}
		for i := range refs {
			if refs[i].Field != wantFields[i] || refs[i].Root != wantRoots[i] {
				t.Fatalf("iter %d: refs[%d] = {Field:%q Root:%q}, want {Field:%q Root:%q}",
					iter, i, refs[i].Field, refs[i].Root, wantFields[i], wantRoots[i])
			}
		}
	}
}

// TestCheckPlanStrict_DeterministicFieldAttribution pins the second
// half of F059: when the same undefined root appears in multiple env
// keys, the dedup must always attribute it to the lowest sorted key
// (env.A), never wobble to env.B.
func TestCheckPlanStrict_DeterministicFieldAttribution(t *testing.T) {
	newPlan := func() *Plan {
		return &Plan{
			Steps: []config.Step{
				{
					Shell: &config.ShellAction{Cmd: "echo hi"},
					Env: map[string]string{
						"ZEBRA": "{{ shared }}",
						"APPLE": "{{ shared }}",
						"MANGO": "{{ shared }}",
					},
				},
			},
		}
	}
	for iter := 0; iter < 50; iter++ {
		refs := CheckPlanStrict(newPlan())
		if len(refs) != 1 {
			t.Fatalf("iter %d: expected 1 deduped ref, got %d: %+v", iter, len(refs), refs)
		}
		if refs[0].Field != "env.APPLE" {
			t.Fatalf("iter %d: Field = %q, want env.APPLE (lowest sorted key wins dedup)", iter, refs[0].Field)
		}
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
