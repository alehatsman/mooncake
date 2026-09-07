package register

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// observePrefix identifies the read-only probe family.
const observePrefix = "observe."

// A read-only probe has nothing to undo, which is not the same as being
// undoable. Every observe.* handler used to implement Reverse as `return nil,
// nil`, which made the Reverser type assertion succeed and made
// `mooncake actions list` report REVERSE=yes for nine pure reads.
//
// The registry derives ImplementsReverse from that assertion, so the only way
// to tell the truth is to not implement the method. This guards that.
func TestObserveHandlersAreNotReversers(t *testing.T) {
	for _, meta := range actions.GlobalRegistry().List() {
		if !strings.HasPrefix(meta.Name, observePrefix) {
			continue
		}
		if meta.ImplementsReverse {
			t.Errorf("%s reports ImplementsReverse=true; a read-only probe must not "+
				"implement Reverse (a no-op `return nil, nil` still satisfies the "+
				"interface and makes `actions list` lie)", meta.Name)
		}
	}
}

// Reversible is the "would a rollback do anything useful?" signal, not the
// Reverser type assertion — see the CostEstimate.Reversible doc. For the
// observe family both answers are false, and both must stay false.
func TestObserveCostIsNotReversible(t *testing.T) {
	reg := actions.GlobalRegistry()
	for _, meta := range reg.List() {
		if !strings.HasPrefix(meta.Name, observePrefix) {
			continue
		}
		handler, ok := reg.Get(meta.Name)
		if !ok {
			t.Fatalf("registry.Get(%q): not found", meta.Name)
		}
		coster, ok := handler.(actions.Coster)
		if !ok {
			t.Errorf("%s does not implement Cost; every observe handler should", meta.Name)
			continue
		}
		est, err := coster.Cost(nil, &config.Step{})
		if err != nil {
			t.Errorf("%s Cost(): %v", meta.Name, err)
			continue
		}
		if est.Reversible {
			t.Errorf("%s reports Cost().Reversible=true; a read has nothing to undo", meta.Name)
		}
		if est.Resources != 0 || est.Bytes != 0 {
			t.Errorf("%s reports Resources=%d Bytes=%d; a pure read mutates nothing",
				meta.Name, est.Resources, est.Bytes)
		}
	}
}

// The observe family is the one place the ABI is deliberately specialized:
// read-only, no mutation. Guard the whole shape, not just Reverse.
func TestObserveHandlersDeclareReadOnlyPermissions(t *testing.T) {
	reg := actions.GlobalRegistry()
	found := 0
	for _, meta := range reg.List() {
		if !strings.HasPrefix(meta.Name, observePrefix) {
			continue
		}
		found++
		handler, ok := reg.Get(meta.Name)
		if !ok {
			t.Fatalf("registry.Get(%q): not found", meta.Name)
		}
		permitter, ok := handler.(actions.Permitter)
		if !ok {
			t.Errorf("%s does not implement Permissions; every observe handler should", meta.Name)
			continue
		}
		perms := permitter.Permissions(&config.Step{})
		if len(perms.FilesystemWrite) > 0 || perms.Sudo {
			t.Errorf("%s claims write/sudo permissions; observe.* is read-only (%+v)",
				meta.Name, perms)
		}
	}
	if found == 0 {
		t.Fatal("no observe.* handlers registered; the guard is not testing anything")
	}
}
