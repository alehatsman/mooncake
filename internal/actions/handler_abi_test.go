package actions

import (
	"errors"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// minimalHandler implements only the base Handler contract — NO
// spec-22 sub-interfaces. Used to assert that the Resolve* defaults
// kick in cleanly. If any default ever changes shape, this test fails
// first.
type minimalHandler struct {
	name string
}

func (m *minimalHandler) Metadata() ActionMetadata {
	return ActionMetadata{Name: m.name}
}
func (m *minimalHandler) Validate(_ *config.Step) error           { return nil }
func (m *minimalHandler) Run(_ Context, _ *config.Step) (Result, error) {
	return nil, nil
}

// richHandler implements every spec-22 sub-interface with distinctive
// return values, so each "implementation supersedes default" assertion
// has something specific to check against.
type richHandler struct {
	minimalHandler
	diffOp     Operation
	revertStep *config.Step
	revertErr  error
	cost       CostEstimate
	perms      PermissionSet
}

func (r *richHandler) Diff(_ Context, _ *config.Step) (Diff, error) {
	return Diff{Operation: r.diffOp, Resource: ResourceRef{Kind: ResourceFile, Identifier: "/tmp/x"}}, nil
}
func (r *richHandler) Reverse(_ Context, _ *config.Step, _ Result) (*config.Step, error) {
	return r.revertStep, r.revertErr
}
func (r *richHandler) Cost(_ Context, _ *config.Step) (CostEstimate, error) {
	return r.cost, nil
}
func (r *richHandler) Permissions(_ *config.Step) PermissionSet { return r.perms }

// reverserOnlyHandler implements only Reverser, to verify that
// ResolveCoster's default reflects Reversible=true when the handler
// also implements Reverser even without an explicit Cost impl. This
// is the subtle spec-22 default semantics — it should NOT require
// implementing Coster to get accurate Reversible reporting.
type reverserOnlyHandler struct {
	minimalHandler
}

func (r *reverserOnlyHandler) Reverse(_ Context, _ *config.Step, _ Result) (*config.Step, error) {
	return &config.Step{Name: "reverse"}, nil
}

// ----- Default behavior on minimal handler ----------------------------------

func TestResolveDiffer_DefaultIsCoarseUpdate(t *testing.T) {
	d := ResolveDiffer(&minimalHandler{name: "fake"})
	got, err := d.Diff(nil, nil)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if got.Operation != OpUpdate {
		t.Errorf("default Operation = %q, want %q", got.Operation, OpUpdate)
	}
	if got.Resource.Kind != ResourceOther {
		t.Errorf("default Resource.Kind = %q, want %q", got.Resource.Kind, ResourceOther)
	}
	if got.Resource.Identifier != "fake" {
		t.Errorf("default Resource.Identifier = %q, want handler name 'fake'", got.Resource.Identifier)
	}
	if len(got.Lines) != 0 {
		t.Errorf("default Lines should be empty, got %d", len(got.Lines))
	}
}

func TestResolveReverser_DefaultIsNoOp(t *testing.T) {
	r := ResolveReverser(&minimalHandler{name: "fake"})
	step, err := r.Reverse(nil, nil, nil)
	if err != nil {
		t.Errorf("default Reverse returned err = %v, want nil (absence ≠ irreversible)", err)
	}
	if step != nil {
		t.Errorf("default Reverse returned step %+v, want nil", step)
	}
}

func TestResolveCoster_DefaultReflectsReversibility(t *testing.T) {
	// Handler without Reverser → default Coster reports Reversible=false.
	c := ResolveCoster(&minimalHandler{name: "fake"})
	got, err := c.Cost(nil, nil)
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if got.Reversible {
		t.Error("default Coster on Reverser-less handler reported Reversible=true, want false")
	}
	if got.Risk != 5 {
		t.Errorf("default Risk = %d, want 5", got.Risk)
	}
	if got.Resources != -1 || got.Bytes != -1 {
		t.Errorf("default Resources/Bytes = %d/%d, want -1/-1 for unknown", got.Resources, got.Bytes)
	}

	// Handler WITH Reverser → default Coster reports Reversible=true,
	// even though Coster itself isn't implemented.
	c2 := ResolveCoster(&reverserOnlyHandler{})
	got2, _ := c2.Cost(nil, nil)
	if !got2.Reversible {
		t.Error("default Coster on Reverser-only handler reported Reversible=false, want true")
	}
}

func TestResolvePermitter_DefaultIsEmpty(t *testing.T) {
	p := ResolvePermitter(&minimalHandler{name: "fake"})
	got := p.Permissions(nil)
	if got.Sudo {
		t.Error("default PermissionSet.Sudo = true, want false")
	}
	if got.Network {
		t.Error("default PermissionSet.Network = true, want false")
	}
	if len(got.RequiredBinaries) != 0 || len(got.FilesystemWrite) != 0 || len(got.Notes) != 0 {
		t.Errorf("default PermissionSet should be empty, got %+v", got)
	}
}

// ----- Native implementation supersedes default ----------------------------

func TestResolve_NativeImplementationWins(t *testing.T) {
	wantStep := &config.Step{Name: "undo"}
	wantPerms := PermissionSet{Sudo: true, RequiredBinaries: []string{"systemctl"}}
	wantCost := CostEstimate{Resources: 42, Bytes: 1024, Reversible: true, Risk: 8}

	h := &richHandler{
		minimalHandler: minimalHandler{name: "rich"},
		diffOp:         OpDelete,
		revertStep:     wantStep,
		cost:           wantCost,
		perms:          wantPerms,
	}

	d := ResolveDiffer(h)
	gotDiff, _ := d.Diff(nil, nil)
	if gotDiff.Operation != OpDelete {
		t.Errorf("Diff.Operation = %q, want OpDelete (native impl), got default OpUpdate?", gotDiff.Operation)
	}
	if gotDiff.Resource.Kind != ResourceFile {
		t.Errorf("Diff.Resource.Kind = %q, want ResourceFile (native impl)", gotDiff.Resource.Kind)
	}

	r := ResolveReverser(h)
	gotStep, gotErr := r.Reverse(nil, nil, nil)
	if gotErr != nil {
		t.Errorf("Reverse err = %v, want nil", gotErr)
	}
	if gotStep != wantStep {
		t.Errorf("Reverse step = %+v, want %+v (native impl)", gotStep, wantStep)
	}

	c := ResolveCoster(h)
	gotCost, _ := c.Cost(nil, nil)
	if gotCost != wantCost {
		t.Errorf("Cost = %+v, want %+v (native impl)", gotCost, wantCost)
	}

	p := ResolvePermitter(h)
	gotPerms := p.Permissions(nil)
	if !gotPerms.Sudo {
		t.Error("Permissions.Sudo = false, want true (native impl)")
	}
	if len(gotPerms.RequiredBinaries) != 1 || gotPerms.RequiredBinaries[0] != "systemctl" {
		t.Errorf("Permissions.RequiredBinaries = %v, want [systemctl] (native impl)", gotPerms.RequiredBinaries)
	}
}

// TestReverser_IrreversibleSignal verifies the (nil, error) shape used
// by irreversible handlers — distinct from the (nil, nil) "no reverse
// needed" default. This is the contract that transactions rely on for
// surfacing irreversible steps to the user.
func TestReverser_IrreversibleSignal(t *testing.T) {
	irreversible := errors.New("shell action is irreversible by design")
	h := &richHandler{
		minimalHandler: minimalHandler{name: "shell"},
		revertErr:      irreversible,
	}
	r := ResolveReverser(h)
	step, err := r.Reverse(nil, nil, nil)
	if step != nil {
		t.Errorf("step = %+v, want nil for irreversible signal", step)
	}
	if !errors.Is(err, irreversible) {
		t.Errorf("err = %v, want sentinel passed through", err)
	}
}

// ----- Is* capability checks -------------------------------------------------

func TestIs_CapabilityChecks(t *testing.T) {
	min := &minimalHandler{name: "min"}
	rich := &richHandler{minimalHandler: minimalHandler{name: "rich"}}

	if IsDiffer(min) {
		t.Error("IsDiffer(minimal) = true, want false")
	}
	if !IsDiffer(rich) {
		t.Error("IsDiffer(rich) = false, want true")
	}
	if IsReverser(min) {
		t.Error("IsReverser(minimal) = true, want false")
	}
	if !IsReverser(rich) {
		t.Error("IsReverser(rich) = false, want true")
	}
	if IsCoster(min) {
		t.Error("IsCoster(minimal) = true, want false")
	}
	if !IsCoster(rich) {
		t.Error("IsCoster(rich) = false, want true")
	}
	if IsPermitter(min) {
		t.Error("IsPermitter(minimal) = true, want false")
	}
	if !IsPermitter(rich) {
		t.Error("IsPermitter(rich) = false, want true")
	}
}

// ----- Default coster reversibility lock-in ---------------------------------

// TestDefaultCoster_ReversibilityCapturedAtConstruction guards the
// implementation detail that defaultCoster's `reversible` field is set
// when ResolveCoster runs, not lazily. This matters because if the
// underlying Handler is wrapped post-construction (it isn't today, but
// might be in future), we'd want the value frozen at registration.
func TestDefaultCoster_ReversibilityCapturedAtConstruction(t *testing.T) {
	c := ResolveCoster(&minimalHandler{name: "noop"})
	dc, ok := c.(defaultCoster)
	if !ok {
		t.Fatalf("ResolveCoster on minimal handler returned %T, want defaultCoster", c)
	}
	if dc.reversible {
		t.Error("defaultCoster.reversible = true, want false")
	}
}
