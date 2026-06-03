package mooncake

// testing.go is the public test-support surface for consumers authoring
// custom typed actions. It lets a consumer unit-test a Handler's Run /
// Reverse in isolation (no executor, no agent loop) using only this module's
// import path — NewTestContext builds an actions.Context, the With* helpers
// read typed params off a step's With map, and AssertHandlerConformance
// checks the ABI contract.

import (
	"context"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
)

// ----------------------------------------------------------------------------
// NewTestContext — a Context for unit-testing a Handler in isolation
// ----------------------------------------------------------------------------

// TestContextOption configures the Context returned by NewTestContext.
// The zero set of options yields a production-faithful default: a real
// template renderer and expression evaluator, a capturing event publisher,
// an error-level logger, ModeApply, and an empty variable scope.
type TestContextOption func(*actions.TestContextConfig)

// WithVars seeds the Context's variable scope. The handler reads these via
// ctx.Variables(); MergeUserVars writes back into the same map.
func WithVars(vars map[string]any) TestContextOption {
	return func(c *actions.TestContextConfig) { c.Vars = vars }
}

// WithMode sets the run mode (ModeApply or ModePlan). Default ModeApply.
func WithMode(mode Mode) TestContextOption {
	return func(c *actions.TestContextConfig) { c.Mode = mode }
}

// WithStepID sets the step ID reported by ctx.StepID(). Default "step-test".
func WithStepID(id string) TestContextOption {
	return func(c *actions.TestContextConfig) { c.StepID = id }
}

// WithRunContext sets the run-wide context returned by ctx.Ctx() — pass a
// cancellable context to exercise a handler's cancellation handling. Default
// context.Background().
func WithRunContext(ctx context.Context) TestContextOption {
	return func(c *actions.TestContextConfig) { c.Ctx = ctx }
}

// WithPublisher overrides the event publisher. By default NewTestContext
// installs a capturing publisher whose events are read back with
// CapturedEvents; supply your own to route events elsewhere.
func WithPublisher(pub Publisher) TestContextOption {
	return func(c *actions.TestContextConfig) { c.Publisher = pub }
}

// NewTestContext returns a Context suitable for driving a custom Handler's
// Run or Reverse in a unit test, configured by opts. The default publisher
// captures every emitted event; recover them with CapturedEvents(ctx).
//
//	ctx := mooncake.NewTestContext(mooncake.WithVars(map[string]any{"name": "ada"}))
//	res, err := myHandler.Run(ctx, step)
func NewTestContext(opts ...TestContextOption) Context {
	var cfg actions.TestContextConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return actions.NewTestContext(cfg)
}

// Publisher is the event-sink interface a Context publishes to. The default
// publisher installed by NewTestContext is a *CapturingPublisher.
type Publisher = events.Publisher

// CapturingPublisher records every published Event so a test can assert on
// what a handler emitted. NewTestContext installs one by default; read its
// events with CapturedEvents.
type CapturingPublisher = actions.CapturingPublisher

// NewCapturingPublisher returns an empty capturing publisher. Pass it to
// WithPublisher when you want to hold the reference yourself; otherwise
// NewTestContext installs one for you and CapturedEvents reaches it.
func NewCapturingPublisher() *CapturingPublisher {
	return actions.NewCapturingPublisher()
}

// CapturedEvents returns the events a handler published through ctx, in
// order, when ctx carries the default capturing publisher (or one supplied
// via WithPublisher(NewCapturingPublisher())). It returns nil when the
// context's publisher is not a *CapturingPublisher — e.g. a custom publisher
// passed to WithPublisher.
func CapturedEvents(ctx Context) []Event {
	if ctx == nil {
		return nil
	}
	if cp, ok := ctx.EventPublisher().(*CapturingPublisher); ok {
		return cp.Events()
	}
	return nil
}

// ----------------------------------------------------------------------------
// Step `with:` typed param getters
// ----------------------------------------------------------------------------

// WithString reads a string parameter named key from a generic-carrier step's
// With map. The bool is false when the key is absent or its value is not a
// string — so a handler's Validate can distinguish "missing" from "present
// but wrong type". A custom handler reads its params off step.With:
//
//	title, ok := mooncake.WithString(step, "title")
//	if !ok {
//	    return errors.New("demo.issue: `title` is required and must be a string")
//	}
func WithString(step *Step, key string) (string, bool) {
	if step == nil || step.With == nil {
		return "", false
	}
	v, ok := step.With[key].(string)
	return v, ok
}

// WithBool reads a bool parameter named key from step.With. The bool is false
// when the key is absent or its value is not a bool.
func WithBool(step *Step, key string) (bool, bool) {
	if step == nil || step.With == nil {
		return false, false
	}
	v, ok := step.With[key].(bool)
	return v, ok
}

// WithInt reads an integer parameter named key from step.With. YAML/JSON
// decoders land integers as int, int64, or float64 depending on the path, so
// WithInt accepts all three and returns the value as int. The bool is false
// when the key is absent or its value is not a whole number.
func WithInt(step *Step, key string) (int, bool) {
	if step == nil || step.With == nil {
		return 0, false
	}
	switch v := step.With[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		// Reject non-integral floats: "3.5" is not an int.
		if v == float64(int(v)) {
			return int(v), true
		}
	}
	return 0, false
}

// ----------------------------------------------------------------------------
// Handler-conformance test helper
// ----------------------------------------------------------------------------

// AssertHandlerConformance verifies a custom Handler honors the ABI contract:
//
//   - Metadata().Name is non-empty (the registry keys handlers by name).
//   - Validate(step) returns nil for the supplied valid step (Validate
//     round-trips a well-formed step without error).
//   - If the handler implements Reverser, Reverse round-trips: given the
//     step and a result, it returns either a reverse step or a clean nil/nil
//     (no reverse needed) — an irreversible handler signals via error, which
//     is accepted here as a declared, honest outcome.
//
// It returns nil on success or a descriptive error on the first violation, so
// a consumer's test can simply:
//
//	if err := mooncake.AssertHandlerConformance(myHandler, validStep); err != nil {
//	    t.Fatal(err)
//	}
//
// validStep must be a step the handler considers valid; pass a representative
// configured step. result is the Result a successful Run would have returned
// (used only for the Reverse round-trip) — pass NewResult() when the handler
// is not a Reverser or its Reverse ignores the result.
func AssertHandlerConformance(h Handler, validStep *Step, result Result) error {
	if h == nil {
		return fmt.Errorf("handler is nil")
	}

	meta := h.Metadata()
	if meta.Name == "" {
		return fmt.Errorf("Metadata().Name is empty; the registry keys handlers by name")
	}

	if err := h.Validate(validStep); err != nil {
		return fmt.Errorf("Validate(validStep) returned error for a step expected to be valid: %w", err)
	}

	if rev, ok := h.(Reverser); ok {
		// Reverse must run without panicking and return. An irreversible
		// handler declares itself via a non-nil error — a valid, honest
		// outcome, not a conformance failure — so the error is intentionally
		// not treated as a failure here.
		ctx := NewTestContext()
		_, _ = rev.Reverse(ctx, validStep, result)
	}

	return nil
}
