//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Pkg reverse tests skip the Run path because it would shell out
// to dpkg/rpm/brew/etc. Instead they construct PkgReverseInfo by
// hand — the same struct installPackages/removePackages populate
// — and assert that Reverse builds the correct inverse Step.
// `result := executor.NewResult(); result.ReverseData = &PkgReverseInfo{...}`
// is the contract Run upholds.

func resultWith(info *PkgReverseInfo) *executor.Result {
	r := executor.NewResult()
	r.ReverseData = info
	return r
}

// TestReverse_PresentInstalledTwo: apply installed 2 of 3 listed
// packages (one was already present). Reverse must remove only the
// 2 we installed.
func TestReverse_PresentInstalledTwo(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Names: []string{"git", "vim", "curl"},
			State: statePresent,
		},
	}
	result := resultWith(&PkgReverseInfo{
		AppliedState: statePresent,
		Manager:      "apt",
		Mutated:      []string{"vim", "curl"}, // git was already installed
	})

	h := &Handler{}
	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.Pkg == nil {
		t.Fatal("reverse step has no Pkg payload")
	}
	if reverseStep.Pkg.State != stateAbsent {
		t.Errorf("reverse state = %q, want %q", reverseStep.Pkg.State, stateAbsent)
	}
	if reverseStep.Pkg.Manager != "apt" {
		t.Errorf("reverse manager = %q, want apt (pinned from capture)", reverseStep.Pkg.Manager)
	}
	got := strings.Join(reverseStep.Pkg.Names, ",")
	if got != "vim,curl" {
		t.Errorf("reverse names = %q, want \"vim,curl\" (only the 2 we installed)", got)
	}
}

// TestReverse_AbsentRemovedAll: apply removed 2 packages, both were
// installed. Reverse must reinstall both.
func TestReverse_AbsentRemovedAll(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Names: []string{"htop", "tmux"},
			State: stateAbsent,
		},
	}
	result := resultWith(&PkgReverseInfo{
		AppliedState: stateAbsent,
		Manager:      "brew",
		Mutated:      []string{"htop", "tmux"},
	})

	h := &Handler{}
	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.Pkg.State != statePresent {
		t.Errorf("reverse state = %q, want %q", reverseStep.Pkg.State, statePresent)
	}
	if reverseStep.Pkg.Manager != "brew" {
		t.Errorf("reverse manager = %q, want brew", reverseStep.Pkg.Manager)
	}
	got := strings.Join(reverseStep.Pkg.Names, ",")
	if got != "htop,tmux" {
		t.Errorf("reverse names = %q, want \"htop,tmux\"", got)
	}
}

// TestReverse_NoMutation_IsNoOp: apply touched nothing (all packages
// were already in the desired state). Reverse returns (nil, nil)
// per the Reverser contract — there's nothing to undo.
func TestReverse_NoMutation_IsNoOp(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Names: []string{"git"},
			State: statePresent,
		},
	}
	result := resultWith(&PkgReverseInfo{
		AppliedState: statePresent,
		Manager:      "apt",
		Mutated:      nil, // git was already installed; apply was a no-op
	})

	h := &Handler{}
	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse should not error for empty mutation: %v", err)
	}
	if reverseStep != nil {
		t.Errorf("Reverse should be (nil, nil) for no-op apply; got step %+v", reverseStep)
	}
}

// TestReverse_UpgradeRefused: Upgrade=true cannot be reversed
// without a pre-apply version snapshot. Refusal is explicit.
func TestReverse_UpgradeRefused(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Upgrade: true,
		},
	}
	result := resultWith(&PkgReverseInfo{AppliedState: statePresent, Manager: "apt"})

	h := &Handler{}
	reverseStep, err := h.Reverse(nil, step, result)
	if err == nil {
		t.Fatal("Reverse should refuse Upgrade=true; got nil error")
	}
	if reverseStep != nil {
		t.Errorf("Reverse returned a step despite refusal: %+v", reverseStep)
	}
	if !strings.Contains(err.Error(), "upgrade=true") {
		t.Errorf("error %q should mention upgrade=true", err.Error())
	}
}

// TestReverse_LatestRefused: state=latest with mutations was an
// upgrade path; we don't capture pre-versions so we can't downgrade.
func TestReverse_LatestRefused(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Names: []string{"vim"},
			State: stateLatest,
		},
	}
	result := resultWith(&PkgReverseInfo{
		AppliedState: stateLatest, // installPackages sets this when upgrade=true
		Manager:      "apt",
		Mutated:      []string{"vim"},
	})

	h := &Handler{}
	_, err := h.Reverse(nil, step, result)
	if err == nil {
		t.Fatal("Reverse should refuse state=latest")
	}
	if !strings.Contains(err.Error(), "state=latest") {
		t.Errorf("error %q should mention state=latest", err.Error())
	}
}

func TestPkgHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}
