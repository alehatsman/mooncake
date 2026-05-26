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

// TestBuildPrompt_LastStepStdout_Rendered confirms the user message
// includes the captured stdout block when LastIteration.LastStepStdout
// is populated. The block is what unblocks step-style loops: without
// it the model re-proposes the same diagnostic step forever.
func TestBuildPrompt_LastStepStdout_Rendered(t *testing.T) {
	const sentinel = "M docs-working/CHANGES.md"
	_, userPrompt, err := BuildPrompt(PlanInput{
		Goal:     "show git status",
		Snapshot: []byte(`{}`),
		LastIteration: &IterationSummary{
			Iteration:      1,
			PlanHash:       "abc123",
			Status:         "success",
			LastStepStdout: "On branch master\n" + sentinel + "\n",
		},
		Style: StyleStep,
	})
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(userPrompt, "Last Step Stdout (last 4 KB):") {
		t.Errorf("user prompt missing Last Step Stdout header; got:\n%s", userPrompt)
	}
	if !strings.Contains(userPrompt, sentinel) {
		t.Errorf("user prompt should embed captured stdout; got:\n%s", userPrompt)
	}
}

// TestBuildPrompt_LastStepStdout_OmittedWhenEmpty pins the absence
// path. A first iteration (LastIteration == nil) or an iteration that
// ran no cmd/shell steps must NOT print a stray empty-block header,
// which would confuse the model into expecting output that doesn't
// exist.
func TestBuildPrompt_LastStepStdout_OmittedWhenEmpty(t *testing.T) {
	t.Run("LastIteration nil", func(t *testing.T) {
		_, userPrompt, err := BuildPrompt(PlanInput{
			Goal:     "do thing",
			Snapshot: []byte(`{}`),
			Style:    StyleStep,
		})
		if err != nil {
			t.Fatalf("BuildPrompt: %v", err)
		}
		if strings.Contains(userPrompt, "Last Step Stdout") {
			t.Errorf("first iteration should not render Last Step Stdout block; got:\n%s", userPrompt)
		}
	})

	t.Run("LastStepStdout empty", func(t *testing.T) {
		_, userPrompt, err := BuildPrompt(PlanInput{
			Goal:     "do thing",
			Snapshot: []byte(`{}`),
			LastIteration: &IterationSummary{
				Iteration: 1,
				PlanHash:  "abc",
				Status:    "success",
				// LastStepStdout intentionally empty.
			},
			Style: StyleStep,
		})
		if err != nil {
			t.Fatalf("BuildPrompt: %v", err)
		}
		if strings.Contains(userPrompt, "Last Step Stdout") {
			t.Errorf("empty LastStepStdout should not render the block; got:\n%s", userPrompt)
		}
	})
}

// TestBuildSystemPrompt_StyleStep_LastStdoutHint pins the step-style
// fragment update: the TASK STYLE block must now coach the model on
// the "empty plan when stdout answers the goal" termination signal.
// Without this hint, even with the captured-stdout block in the user
// message, smaller models tend to re-propose the same diagnostic step.
func TestBuildSystemPrompt_StyleStep_LastStdoutHint(t *testing.T) {
	got, err := buildSystemPrompt(StyleStep)
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}
	mustContain(t, got, "If LAST STEP STDOUT above answers the goal")
	mustContain(t, got, "do not re-propose the same diagnostic step")
}

// TestBuildSystemPrompt_StylePlan_NoLastStdoutHint pins the deliberate
// asymmetry: plan-style emits the whole plan in one turn, so per-
// iteration stdout isn't part of its mental model and the hint must
// stay out of promptStylePlan (avoid prompt-drift between styles).
func TestBuildSystemPrompt_StylePlan_NoLastStdoutHint(t *testing.T) {
	got, err := buildSystemPrompt(StylePlan)
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}
	if strings.Contains(got, "If LAST STEP STDOUT above answers the goal") {
		t.Errorf("plan-style prompt should not carry the step-style stdout hint; got:\n%s", got)
	}
}
