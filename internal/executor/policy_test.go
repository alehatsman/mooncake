package executor

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// shellStep builds a minimal step whose DetermineActionType is "shell"
// — the canonical escape-hatch action a run policy most wants to deny.
func shellStep() *config.Step {
	return &config.Step{Name: "run something", Shell: &config.ShellAction{Cmd: "echo hi"}}
}

func TestPolicy_NilEnforcesNothing(t *testing.T) {
	var p *Policy
	if err := p.check(shellStep(), actions.PermissionSet{Network: true}, 10); err != nil {
		t.Fatalf("nil policy must allow every step, got: %v", err)
	}
	if !p.IsZero() {
		t.Fatal("nil policy should report IsZero")
	}
}

func TestPolicy_ZeroValueEnforcesNothing(t *testing.T) {
	p := &Policy{}
	if !p.IsZero() {
		t.Fatal("empty policy should report IsZero")
	}
	if err := p.check(shellStep(), actions.PermissionSet{Network: true}, 10); err != nil {
		t.Fatalf("empty policy must allow every step, got: %v", err)
	}
}

func TestPolicy_DenylistBlocksShell(t *testing.T) {
	p := &Policy{DeniedActions: []string{"shell", "cmd"}}
	err := p.check(shellStep(), actions.PermissionSet{}, defaultPolicyRisk)
	if err == nil {
		t.Fatal("expected shell to be denied by the denylist")
	}
	if !strings.Contains(err.Error(), "denylist") || !strings.Contains(err.Error(), "shell") {
		t.Fatalf("error should name the rule and action, got: %v", err)
	}
}

func TestPolicy_AllowlistDeniesUnlisted(t *testing.T) {
	// Allowlist of typed file actions — shell is not in it.
	p := &Policy{AllowedActions: []string{"file.write", "text.replace"}}
	err := p.check(shellStep(), actions.PermissionSet{}, defaultPolicyRisk)
	if err == nil {
		t.Fatal("expected shell to be denied — not in the allowlist")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("error should name the allowlist rule, got: %v", err)
	}
}

func TestPolicy_AllowlistPermitsListed(t *testing.T) {
	p := &Policy{AllowedActions: []string{"shell"}}
	if err := p.check(shellStep(), actions.PermissionSet{}, defaultPolicyRisk); err != nil {
		t.Fatalf("shell is allowlisted; should pass, got: %v", err)
	}
}

func TestPolicy_DenylistWinsOverAllowlist(t *testing.T) {
	// Even allowlisted, an action on the denylist is rejected.
	p := &Policy{AllowedActions: []string{"shell"}, DeniedActions: []string{"shell"}}
	err := p.check(shellStep(), actions.PermissionSet{}, defaultPolicyRisk)
	if err == nil || !strings.Contains(err.Error(), "denylist") {
		t.Fatalf("denylist must win over allowlist, got: %v", err)
	}
}

func TestPolicy_DenyNetwork(t *testing.T) {
	p := &Policy{DenyNetwork: true}

	// A step that declares network egress is denied.
	err := p.check(shellStep(), actions.PermissionSet{Network: true}, defaultPolicyRisk)
	if err == nil || !strings.Contains(err.Error(), "network") {
		t.Fatalf("network-declaring step should be denied, got: %v", err)
	}

	// A step with no network declaration passes the network gate.
	if err := p.check(shellStep(), actions.PermissionSet{Network: false}, defaultPolicyRisk); err != nil {
		t.Fatalf("non-network step should pass DenyNetwork, got: %v", err)
	}
}

func TestPolicy_MaxRisk(t *testing.T) {
	p := &Policy{MaxRisk: 6}

	if err := p.check(shellStep(), actions.PermissionSet{}, 6); err != nil {
		t.Fatalf("risk equal to the cap should pass, got: %v", err)
	}
	if err := p.check(shellStep(), actions.PermissionSet{}, 5); err != nil {
		t.Fatalf("risk below the cap should pass, got: %v", err)
	}
	err := p.check(shellStep(), actions.PermissionSet{}, 9)
	if err == nil || !strings.Contains(err.Error(), "risk") {
		t.Fatalf("risk above the cap should be denied, got: %v", err)
	}
}

func TestPolicy_MaxRiskZeroIsNoCap(t *testing.T) {
	p := &Policy{MaxRisk: 0}
	if err := p.check(shellStep(), actions.PermissionSet{}, 10); err != nil {
		t.Fatalf("MaxRisk 0 must disable the cap, got: %v", err)
	}
}

func TestPolicy_DenylistCaseInsensitive(t *testing.T) {
	// A policy authored with odd casing/whitespace must still match the
	// canonical lower-case action type — a casing typo shouldn't silently
	// turn a deny into an allow.
	p := &Policy{DeniedActions: []string{" SHELL "}}
	if err := p.check(shellStep(), actions.PermissionSet{}, defaultPolicyRisk); err == nil {
		t.Fatal("denylist match should be case-insensitive and whitespace-trimmed")
	}
}

func TestPolicy_IsZero(t *testing.T) {
	cases := []struct {
		name string
		p    *Policy
		zero bool
	}{
		{"nil", nil, true},
		{"empty", &Policy{}, true},
		{"allow", &Policy{AllowedActions: []string{"shell"}}, false},
		{"deny", &Policy{DeniedActions: []string{"shell"}}, false},
		{"network", &Policy{DenyNetwork: true}, false},
		{"risk", &Policy{MaxRisk: 3}, false},
	}
	for _, tc := range cases {
		if got := tc.p.IsZero(); got != tc.zero {
			t.Errorf("%s: IsZero = %v, want %v", tc.name, got, tc.zero)
		}
	}
}
