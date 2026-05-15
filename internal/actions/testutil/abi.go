// Package testutil — spec-22 ABI assertion helpers.
//
// Every handler that opts into the extended Handler ABI ends up
// repeating two test patterns:
//
//   - "Reverse refuses with a specific reason" (interface implemented
//     but apply-time capture not yet wired) — git.checkout,
//     git.config, pkg.repo, pkg.hold, os.service, file.unarchive.
//   - "Diff / Reverse / etc. errors on a nil step" — defensive shape
//     every ABI method should honor.
//
// These helpers wrap the assertion plumbing so a handler's ABI
// tests focus on the handler's actual behavior (risk band,
// operation classification, identifier shape) rather than
// reasserting boilerplate.
//
// Compile-time interface checks (`var _ actions.Permitter =
// (*Handler)(nil)`) are NOT generalized into helpers — they're
// idiomatic Go, one line each, and a generic helper would actually
// add ceremony.
package testutil

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// AssertReverseRefuses checks the common "interface implemented as
// explicit refusal" pattern used by handlers whose true reverse
// needs apply-time pre-state capture (git.checkout, git.config,
// pkg.repo, pkg.hold, etc.). Asserts:
//
//   - the returned step is nil
//   - an error is returned
//   - the error message contains wantMsg (typically "not yet
//     implemented" or "irreversible by design")
//
// Pass the (step, err) tuple Reverse returned directly.
func AssertReverseRefuses(t *testing.T, step *config.Step, err error, wantMsg string) {
	t.Helper()
	if step != nil {
		t.Errorf("Reverse must return nil step on refusal; got %+v", step)
	}
	if err == nil {
		t.Fatal("Reverse must return a refusal error")
	}
	if wantMsg != "" && !strings.Contains(err.Error(), wantMsg) {
		t.Errorf("refusal error must mention %q; got: %s", wantMsg, err)
	}
}

// AssertNilStepErrors lifts the three-line "(_, err := h.X(nil,
// nil); err == nil { t.Fatal(...) })" pattern out of every ABI test
// file. Pass a closure that invokes the method under test with a
// nil step and returns just the error:
//
//	testutil.AssertNilStepErrors(t, "Diff", func() error {
//	    _, err := h.Diff(nil, nil)
//	    return err
//	})
func AssertNilStepErrors(t *testing.T, name string, call func() error) {
	t.Helper()
	if err := call(); err == nil {
		t.Fatalf("%s must error on nil step", name)
	}
}
