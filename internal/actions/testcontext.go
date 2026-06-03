package actions

import (
	"context"
	"os"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// TestContext is a self-contained actions.Context for unit-testing a custom
// Handler's Run / Reverse in isolation, without standing up the executor.
//
// It wires the REAL template renderer and expression evaluator (so template
// and when-expression behavior matches production), a capturing event
// publisher (so a test can assert which events a handler emitted), a logger,
// a variables map, and a run Mode. Effects() returns a no-op Performer that
// records nothing — a handler that mutates the filesystem through
// ctx.Effects() will not touch disk under this context, which is the point
// for an isolated unit test.
//
// Construct one with NewTestContext. The public SDK exposes this via
// mooncake.NewTestContext; external consumers never name this type directly.
type TestContext struct {
	vars      map[string]interface{}
	renderer  template.Renderer
	evaluator expression.Evaluator
	publisher events.Publisher
	log       logger.Logger
	mode      Mode
	stepID    string
	performer Performer
	runCtx    context.Context
}

// TestContextConfig is the resolved configuration NewTestContext builds a
// TestContext from. The SDK layer maps its functional options onto this
// struct; defaults are filled in by NewTestContext for any zero field.
type TestContextConfig struct {
	// Vars seeds the variable scope. nil yields an empty map.
	Vars map[string]interface{}
	// Renderer overrides the template renderer. nil yields a real Pongo2
	// renderer.
	Renderer template.Renderer
	// Evaluator overrides the expression evaluator. nil yields a real expr
	// evaluator.
	Evaluator expression.Evaluator
	// Publisher overrides the event publisher. nil yields a capturing
	// publisher (*CapturingPublisher) whose events the caller can read back.
	Publisher events.Publisher
	// Logger overrides the logger. nil yields an error-level logger.
	Logger logger.Logger
	// Mode is the run mode. The zero value (ModeApply) is the default.
	Mode Mode
	// StepID overrides the step ID. "" yields "step-test".
	StepID string
	// Performer overrides the Effects() performer. nil yields a no-op
	// performer that records nothing and touches no filesystem.
	Performer Performer
	// Ctx overrides the run-wide context returned by Ctx(). nil yields
	// context.Background().
	Ctx context.Context
}

// NewTestContext builds a TestContext from cfg, filling any zero field with a
// production-faithful default (real renderer + evaluator, capturing
// publisher, error-level logger, ModeApply). It never returns an error: the
// renderer construction failure (filter registration) falls back to a
// zero-value renderer, mirroring internal/apply's reverseContext, so callers
// in tests don't have to handle a degenerate error path.
func NewTestContext(cfg TestContextConfig) *TestContext {
	vars := cfg.Vars
	if vars == nil {
		vars = map[string]interface{}{}
	}

	renderer := cfg.Renderer
	if renderer == nil {
		if r, err := template.NewPongo2Renderer(); err == nil && r != nil {
			renderer = r
		} else {
			renderer = &template.Pongo2Renderer{}
		}
	}

	evaluator := cfg.Evaluator
	if evaluator == nil {
		evaluator = expression.NewExprEvaluator()
	}

	publisher := cfg.Publisher
	if publisher == nil {
		publisher = NewCapturingPublisher()
	}

	log := cfg.Logger
	if log == nil {
		log = logger.NewLogger(logger.ErrorLevel)
	}

	stepID := cfg.StepID
	if stepID == "" {
		stepID = "step-test"
	}

	runCtx := cfg.Ctx
	if runCtx == nil {
		runCtx = context.Background()
	}

	return &TestContext{
		vars:      vars,
		renderer:  renderer,
		evaluator: evaluator,
		publisher: publisher,
		log:       log,
		mode:      cfg.Mode,
		stepID:    stepID,
		performer: cfg.Performer,
		runCtx:    runCtx,
	}
}

// Template returns the configured renderer.
func (c *TestContext) Template() template.Renderer { return c.renderer }

// Evaluator returns the configured expression evaluator.
func (c *TestContext) Evaluator() expression.Evaluator { return c.evaluator }

// Logger returns the configured logger.
func (c *TestContext) Logger() logger.Logger { return c.log }

// Variables returns the variable scope map.
func (c *TestContext) Variables() map[string]interface{} { return c.vars }

// EventPublisher returns the configured event publisher.
func (c *TestContext) EventPublisher() events.Publisher { return c.publisher }

// Mode returns the configured run mode.
func (c *TestContext) Mode() Mode { return c.mode }

// Effects returns the configured Performer, or a no-op performer bound to the
// current Mode when none was injected.
func (c *TestContext) Effects() Performer {
	if c.performer != nil {
		return c.performer
	}
	return noopContextPerformer{mode: c.mode}
}

// Privileged returns an already-root, no-sudo escalation primitive so a
// handler that shells out under test does not block on a sudo prompt. Tests
// asserting against sudo behavior should construct their own Performer/handler
// double instead.
func (c *TestContext) Privileged() *security.Privileged {
	return &security.Privileged{
		Escalation: security.EscalationReport{
			Available: true,
			Reason:    security.EscalationAvailableRoot,
		},
	}
}

// StepID returns the configured step ID.
func (c *TestContext) StepID() string { return c.stepID }

// MergeUserVars merges vars into the variable scope.
func (c *TestContext) MergeUserVars(vars map[string]interface{}) {
	for k, v := range vars {
		c.vars[k] = v
	}
}

// Ctx returns the configured run-wide context.
func (c *TestContext) Ctx() context.Context { return c.runCtx }

// CapturingPublisher is an events.Publisher that records every published
// event in memory so a test can assert on what a handler emitted. It is the
// default publisher NewTestContext installs when none is supplied.
type CapturingPublisher struct {
	events []events.Event
}

// NewCapturingPublisher returns an empty CapturingPublisher.
func NewCapturingPublisher() *CapturingPublisher {
	return &CapturingPublisher{}
}

// Publish records the event.
func (p *CapturingPublisher) Publish(event events.Event) {
	if p == nil {
		return
	}
	p.events = append(p.events, event)
}

// Events returns the events published so far, in order. The returned slice is
// a copy; mutating it does not affect the publisher.
func (p *CapturingPublisher) Events() []events.Event {
	if p == nil || len(p.events) == 0 {
		return nil
	}
	out := make([]events.Event, len(p.events))
	copy(out, p.events)
	return out
}

// Subscribe is a no-op; the capturing publisher has no subscribers.
func (p *CapturingPublisher) Subscribe(_ events.Subscriber) int { return 0 }

// Unsubscribe is a no-op.
func (p *CapturingPublisher) Unsubscribe(_ int) {}

// Flush is a no-op; events are recorded synchronously.
func (p *CapturingPublisher) Flush() {}

// Close is a no-op.
func (p *CapturingPublisher) Close() {}

// noopContextPerformer satisfies Performer without touching the filesystem. It
// is the default Effects() performer for a TestContext when none is injected.
type noopContextPerformer struct{ mode Mode }

func (p noopContextPerformer) Mode() Mode { return p.mode }

func (p noopContextPerformer) Mkdir(path string, _ os.FileMode, _ PerformerOpts) Effect {
	return Effect{Action: ActionMkdir, Path: path}
}
func (p noopContextPerformer) WriteFile(path string, _ []byte, _ os.FileMode, _ PerformerOpts) Effect {
	return Effect{Action: ActionWriteFile, Path: path}
}
func (p noopContextPerformer) CopyFile(_, dest string, _ os.FileMode, _ PerformerOpts) Effect {
	return Effect{Action: ActionCopyFile, Path: dest}
}
func (p noopContextPerformer) Symlink(_, path string, _ PerformerOpts) Effect {
	return Effect{Action: ActionSymlink, Path: path}
}
func (p noopContextPerformer) Hardlink(_, path string, _ PerformerOpts) Effect {
	return Effect{Action: ActionHardlink, Path: path}
}
func (p noopContextPerformer) Touch(path string, _ os.FileMode, _ PerformerOpts) Effect {
	return Effect{Action: ActionTouch, Path: path}
}
func (p noopContextPerformer) Remove(path string, _ bool, _ PerformerOpts) Effect {
	return Effect{Action: ActionRemove, Path: path}
}
func (p noopContextPerformer) Chmod(path string, _ os.FileMode, _ PerformerOpts) Effect {
	return Effect{Action: ActionChmod, Path: path}
}
func (p noopContextPerformer) Chown(path, _, _ string, _ PerformerOpts) Effect {
	return Effect{Action: ActionChown, Path: path}
}

// compile-time assertions that TestContext satisfies Context and the
// publisher satisfies events.Publisher.
var (
	_ Context          = (*TestContext)(nil)
	_ events.Publisher = (*CapturingPublisher)(nil)
)
