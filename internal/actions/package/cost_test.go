//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_InstallPresent(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{Names: []string{"git", "vim"}, State: statePresent}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 5 {
		t.Errorf("Risk = %d, want 5 (install)", c.Risk)
	}
	if c.Resources != 2 {
		t.Errorf("Resources = %d, want 2 packages", c.Resources)
	}
}

func TestCost_RemoveIsHigherRisk(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{Names: []string{"x"}, State: stateAbsent}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7 (remove)", c.Risk)
	}
}

func TestCost_LatestIsRisk8(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{Names: []string{"x"}, State: stateLatest}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 8 {
		t.Errorf("Risk = %d, want 8 (latest = upgrade existing)", c.Risk)
	}
}

func TestCost_UpgradeAllIsRisk9(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{Upgrade: true}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 9 {
		t.Errorf("Upgrade=true Risk = %d, want 9", c.Risk)
	}
}

func TestPkgHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
