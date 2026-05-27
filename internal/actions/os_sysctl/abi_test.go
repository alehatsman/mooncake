package os_sysctl //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward", Value: 1}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "sysctl" {
		t.Errorf("RequiredBinaries = %v, want [sysctl]", ps.RequiredBinaries)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsUpdate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward", Value: 1}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpUpdate)
	}
	after := d.After.(*actions.SysctlDiff)
	if after.Value != "1" {
		t.Errorf("Value = %q, want '1'", after.Value)
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x", State: "absent"}})
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpDelete)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_RiskIs6(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x"}})
	if c.Risk != 6 {
		t.Errorf("Risk = %d, want 6", c.Risk)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_AddedLineBecomesAbsent(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSysctlReverseInfo{
		Name:               "net.ipv4.ip_forward",
		AppliedState:       "present",
		HadPriorPersist:    false,
		TouchedPersistFile: true,
	}
	rev, err := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsSysctl == nil {
		t.Fatal("Reverse must return an os.sysctl step")
	}
	if rev.OsSysctl.State != "absent" {
		t.Errorf("State = %s, want absent", rev.OsSysctl.State)
	}
	if rev.OsSysctl.Persist == nil || *rev.OsSysctl.Persist != false {
		t.Errorf("Persist must be set to false on remove; got %v", rev.OsSysctl.Persist)
	}
}

func TestReverse_UpdatedLineRestoresPriorValue(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSysctlReverseInfo{
		Name:               "vm.swappiness",
		AppliedState:       "present",
		PriorRuntimeValue:  "60",
		HadPriorRuntime:    true,
		PriorPersistValue:  "60",
		HadPriorPersist:    true,
		TouchedPersistFile: true,
		TouchedRuntime:     true,
	}
	rev, _ := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "vm.swappiness"}}, r)
	if rev == nil || rev.OsSysctl == nil {
		t.Fatal("Reverse must return an os.sysctl step")
	}
	val, _ := rev.OsSysctl.Value.(string)
	if val != "60" {
		t.Errorf("Value = %v, want 60", rev.OsSysctl.Value)
	}
	if rev.OsSysctl.Persist == nil || *rev.OsSysctl.Persist != true {
		t.Errorf("Persist must be true; got %v", rev.OsSysctl.Persist)
	}
	if rev.OsSysctl.Reload == nil || *rev.OsSysctl.Reload != true {
		t.Errorf("Reload must be true when runtime was touched + prior known; got %v", rev.OsSysctl.Reload)
	}
}

func TestReverse_RuntimeOnlyMutationRestoresRuntime(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSysctlReverseInfo{
		Name:              "kernel.shmmax",
		AppliedState:      "present",
		PriorRuntimeValue: "18446744073692774399",
		HadPriorRuntime:   true,
		TouchedRuntime:    true,
		// TouchedPersistFile: false — persist disabled this run.
	}
	rev, _ := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "kernel.shmmax"}}, r)
	if rev == nil || rev.OsSysctl == nil {
		t.Fatal("Reverse must return an os.sysctl step")
	}
	val, _ := rev.OsSysctl.Value.(string)
	if val != "18446744073692774399" {
		t.Errorf("Value = %v, want the prior runtime value", rev.OsSysctl.Value)
	}
	if rev.OsSysctl.Persist == nil || *rev.OsSysctl.Persist != false {
		t.Errorf("Persist must be false on runtime-only reverse; got %v", rev.OsSysctl.Persist)
	}
}

func TestReverse_RuntimeOnlyWithoutPriorErrors(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSysctlReverseInfo{
		Name:           "kernel.unknown",
		AppliedState:   "present",
		TouchedRuntime: true,
		// HadPriorRuntime: false — sysctl -n failed pre-apply.
	}
	_, err := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "kernel.unknown"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when runtime was touched but prior value is unknown")
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	step, err := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x"}}, r)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", step)
	}
}

func TestReverse_NeitherTouchedIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSysctlReverseInfo{Name: "x"}
	step, _ := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x"}}, r)
	if step != nil {
		t.Errorf("Reverse with neither touched must return nil; got %+v", step)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = "wrong"
	_, err := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
