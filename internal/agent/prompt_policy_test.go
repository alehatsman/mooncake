package agent

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/executor"
)

// TestRenderPolicyContract_EmptyForNilOrZero pins the no-op path: a nil
// or zero policy must render "" so an unrestricted run's prompt stays
// byte-identical to the pre-policy shape. Mirrors buildAgentPolicy's
// nil-on-no-flags contract.
func TestRenderPolicyContract_EmptyForNilOrZero(t *testing.T) {
	if got := renderPolicyContract(nil); got != "" {
		t.Errorf("nil policy should render empty; got: %q", got)
	}
	if got := renderPolicyContract(&executor.Policy{}); got != "" {
		t.Errorf("zero policy should render empty; got: %q", got)
	}
}

// TestRenderPolicyContract_AllFields verifies every populated field
// surfaces a contract line. The wording is kept close to
// executor.Policy.check's refusal text so a model reading the contract
// and a model reading a refusal see the same vocabulary.
func TestRenderPolicyContract_AllFields(t *testing.T) {
	got := renderPolicyContract(&executor.Policy{
		AllowedActions: []string{"file.write", "template"},
		DeniedActions:  []string{"shell", "cmd"},
		DenyNetwork:    true,
		MaxRisk:        4,
	})

	mustContain(t, got, "PERMISSIONS CONTRACT")
	mustContain(t, got, "You may ONLY use these action types: file.write, template")
	mustContain(t, got, "You may NOT use these action types: shell, cmd")
	mustContain(t, got, "Network egress is FORBIDDEN")
	mustContain(t, got, "Maximum risk band is 4")
}

// TestRenderPolicyContract_OnlyPopulatedLines confirms each field is
// independent: a deny-only policy renders just the denylist line, no
// stray allowlist/network/risk lines.
func TestRenderPolicyContract_OnlyPopulatedLines(t *testing.T) {
	got := renderPolicyContract(&executor.Policy{DeniedActions: []string{"shell"}})

	mustContain(t, got, "You may NOT use these action types: shell")
	for _, absent := range []string{
		"You may ONLY use",
		"Network egress is FORBIDDEN",
		"Maximum risk band",
	} {
		if strings.Contains(got, absent) {
			t.Errorf("deny-only contract should not contain %q; got:\n%s", absent, got)
		}
	}
}

// TestBuildSystemPrompt_PolicyContractInjected proves the contract block
// lands in the system prompt when a policy is set, and that it sits
// after the action vocabulary (the model reads the full action list,
// then the constraint on which subset it may use).
func TestBuildSystemPrompt_PolicyContractInjected(t *testing.T) {
	got, err := buildSystemPrompt(StylePlan, &executor.Policy{DeniedActions: []string{"shell", "cmd"}}, nil)
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}

	mustContain(t, got, "PERMISSIONS CONTRACT")
	mustContain(t, got, "You may NOT use these action types: shell, cmd")

	actionsIdx := strings.Index(got, "ACTIONS (grouped by category):")
	contractIdx := strings.Index(got, "PERMISSIONS CONTRACT")
	practicesIdx := strings.Index(got, "BEST PRACTICES:")
	if !(actionsIdx >= 0 && contractIdx > actionsIdx && practicesIdx > contractIdx) {
		t.Errorf("contract should sit between the action vocabulary and BEST PRACTICES; actions=%d contract=%d practices=%d", actionsIdx, contractIdx, practicesIdx)
	}
}

// TestBuildSystemPrompt_NoContractWithoutPolicy pins the absence path:
// a nil policy must not emit a PERMISSIONS CONTRACT block, keeping the
// unrestricted prompt unchanged.
func TestBuildSystemPrompt_NoContractWithoutPolicy(t *testing.T) {
	got, err := buildSystemPrompt(StylePlan, nil, nil)
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}
	if strings.Contains(got, "PERMISSIONS CONTRACT") {
		t.Errorf("nil-policy prompt should not carry a PERMISSIONS CONTRACT block; got:\n%s", got)
	}
}

// TestBuildPrompt_PolicyForwarded is the end-to-end check: a Policy on
// PlanInput reaches the rendered system prompt. This is the seam the
// loop relies on (opts.Policy → PlanInput.Policy → contract block).
func TestBuildPrompt_PolicyForwarded(t *testing.T) {
	systemPrompt, _, err := BuildPrompt(PlanInput{
		Goal:     "configure nginx",
		Snapshot: []byte(`{}`),
		Style:    StylePlan,
		Policy:   &executor.Policy{DenyNetwork: true},
	})
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	mustContain(t, systemPrompt, "Network egress is FORBIDDEN")
}
