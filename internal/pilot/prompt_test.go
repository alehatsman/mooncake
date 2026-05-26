package pilot

import (
	"strings"
	"testing"
)

// TestBuildSystemPrompt_StylePlan_Snapshot pins the rendered plan-
// style system prompt's structure. We don't pin the full string
// (the schema chunk legitimately changes when actions are added/
// renamed) but we verify every load-bearing fragment is present and
// the TASK STYLE block is the plan-style one. Prompt drift surfaces
// in PR diff because the assertions stay narrow on the style fragment.
func TestBuildSystemPrompt_StylePlan_Snapshot(t *testing.T) {
	got, err := buildSystemPrompt(StylePlan)
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}

	mustContain(t, got, "You are a Mooncake agent planner.")
	mustContain(t, got, "ACTIONS (grouped by category):")
	mustContain(t, got, "UNIVERSAL STEP FIELDS:")
	mustContain(t, got, "BEST PRACTICES:")
	mustContain(t, got, "CONSTRAINTS:")
	mustContain(t, got, "TASK STYLE: complete plan")
	mustContain(t, got, "Design a complete mooncake YAML plan accomplishing this goal.")
	mustContain(t, got, "Aim for 4–30 steps")
	mustContain(t, got, "`assert:` where useful.")

	if strings.Contains(got, "TASK STYLE: one step at a time") {
		t.Errorf("plan-style prompt should not contain step-style fragment")
	}
}

// TestBuildSystemPrompt_StyleStep_Snapshot verifies the step-style
// TASK STYLE block is appended (and the plan-style one is not).
// Plan §8 decision 3: the step prompt omits the `assert:` hint, so
// we also pin that absence.
func TestBuildSystemPrompt_StyleStep_Snapshot(t *testing.T) {
	got, err := buildSystemPrompt(StyleStep)
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}

	mustContain(t, got, "You are a Mooncake agent planner.")
	mustContain(t, got, "ACTIONS (grouped by category):")
	mustContain(t, got, "TASK STYLE: one step at a time")
	mustContain(t, got, "Propose the NEXT SINGLE action")
	mustContain(t, got, "EXACTLY ONE step")
	mustContain(t, got, "emit an empty plan (the YAML literal `[]`) to signal \"done\".")

	if strings.Contains(got, "TASK STYLE: complete plan") {
		t.Errorf("step-style prompt should not contain plan-style fragment")
	}
	if strings.Contains(got, "Aim for 4–30 steps") {
		t.Errorf("step-style prompt should not advertise multi-step plan length")
	}
	// Plan §8 decision 3: no assert: hint in step prompt. The TASK
	// STYLE block itself must not reference `assert:` — best-practices
	// can still surface "Use assert to verify changes" higher up since
	// that's general guidance, not a step-style instruction.
	styleIdx := strings.Index(got, "TASK STYLE:")
	if styleIdx < 0 {
		t.Fatal("missing TASK STYLE marker")
	}
	if strings.Contains(got[styleIdx:], "assert:") {
		t.Errorf("step-style TASK STYLE block must omit assert: hint per plan §8 decision 3; got: %s", got[styleIdx:])
	}
}

// TestBuildPrompt_StyleStep_UserPromptHasNoMultiStepHint verifies the
// user-message footer no longer prints the multi-step "Example
// format" hint (dropped in favor of the TASK STYLE block).
func TestBuildPrompt_StyleStep_UserPromptHasNoMultiStepHint(t *testing.T) {
	_, userPrompt, err := BuildPrompt(PlanInput{
		Goal:     "install postgres",
		Snapshot: []byte(`{}`),
		Style:    StyleStep,
	})
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if strings.Contains(userPrompt, "Example format:") {
		t.Errorf("user prompt should not carry the legacy Example format hint; got: %s", userPrompt)
	}
}

// TestSelectStyleFragment_UnknownFallsBackToPlan covers the defensive
// switch default: a stray env var or pilot.yml typo should still
// produce a coherent plan-style prompt, not a blank one.
func TestSelectStyleFragment_UnknownFallsBackToPlan(t *testing.T) {
	got := selectStyleFragment(Style("nonsense"))
	if !strings.Contains(got, "TASK STYLE: complete plan") {
		t.Errorf("unknown style should fall back to plan; got: %s", got)
	}
}
