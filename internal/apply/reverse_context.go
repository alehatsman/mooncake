package apply

import (
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// reverseContext is a minimal actions.Context used by
// KernelResult.Reverse() when calling Reverser handlers
// post-run. Reverse implementations should rely only on the
// per-step Result.ReverseData snapshot captured pre-mutation
// (spec-22 phase 5) — they construct the inverse step from the
// recorded state, not from live execution context. This context
// therefore returns safe no-op implementations for every
// Context method.
//
// If a handler's Reverse() reaches for the template renderer,
// evaluator, variables, or Effects(), that's a handler bug —
// Reverse must be pure on its inputs (step + result.ReverseData)
// so it can be replayed across runs.
type reverseContext struct{}

func newReverseContext() *reverseContext { return &reverseContext{} }

// Mode reports the kernel mode. Reverse runs after the original
// run completed; semantically it's an apply-mode operation
// (the inverse step, if scheduled, will mutate state).
func (c *reverseContext) Mode() actions.Mode { return actions.ModeApply }

func (c *reverseContext) GetTemplate() template.Renderer {
	// Pongo2 renderer construction can fail; ignore — Reverse should
	// not use the renderer. Returning nil would crash any handler
	// that did reach for it; returning the zero-value renderer is
	// safer.
	r, _ := template.NewPongo2Renderer()
	return r
}
func (c *reverseContext) GetEvaluator() expression.Evaluator   { return expression.NewExprEvaluator() }
func (c *reverseContext) GetLogger() logger.Logger             { return logger.NewLogger(logger.InfoLevel) }
func (c *reverseContext) GetVariables() map[string]interface{} { return map[string]interface{}{} }
func (c *reverseContext) GetEventPublisher() events.Publisher  { return events.NewPublisher() }
func (c *reverseContext) GetCurrentStepID() string             { return "reverse" }
func (c *reverseContext) Effects() actions.Performer           { return reverseNoopPerformer{} }
func (c *reverseContext) Privileged() *security.Privileged {
	// Reverse runs after the original — give it an already-root
	// no-sudo primitive so any handler that misbehaves and shells
	// out doesn't get blocked on a sudo prompt.
	return &security.Privileged{
		Escalation: security.EscalationReport{Available: true, Reason: security.EscalationAvailableRoot},
	}
}

func (c *reverseContext) MergeUserVars(_ map[string]interface{}) {}
func (c *reverseContext) SetChanged(_ bool)                      {}
func (c *reverseContext) SetStdout(_ string)                     {}
func (c *reverseContext) SetStderr(_ string)                     {}
func (c *reverseContext) SetFailed(_ bool)                       {}
func (c *reverseContext) SetData(_ map[string]interface{})       {}

// reverseNoopPerformer satisfies actions.Performer with no-op
// returns. Reverse handlers should not call Effects(); this guard
// exists so a misbehaving handler doesn't NPE.
type reverseNoopPerformer struct{}

func (reverseNoopPerformer) Mode() actions.Mode { return actions.ModeApply }
func (reverseNoopPerformer) Mkdir(string, os.FileMode, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) WriteFile(string, []byte, os.FileMode, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) CopyFile(string, string, os.FileMode, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) Symlink(string, string, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) Hardlink(string, string, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) Touch(string, os.FileMode, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) Remove(string, bool, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) Chmod(string, os.FileMode, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
func (reverseNoopPerformer) Chown(string, string, string, actions.PerformerOpts) actions.Effect {
	return actions.Effect{}
}
