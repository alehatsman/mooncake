package plan

// planner_trycatch.go implements the spec-23 §2 try / catch / finally
// expansion. The parser deposits a Step with non-empty Try children +
// optional Catch + optional Finally; this file turns that into plan-
// level steps with TryParent linkage and a TryRole tag on each child.
//
// What this file does NOT do: any execution semantics. The executor
// side lives in internal/executor/trycatch.go and ExecuteSteps' try-
// block error-handling branch.

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/config"
)

// expandTry emits one parent plan step (no action; carries the
// compound's metadata so plan output renders the wrapper) followed by
// each branch's children as sibling steps with TryParent = parent.ID
// and TryRole = "try" | "catch" | "finally".
//
// Order in the emitted plan: parent, try children, catch children,
// finally children. The executor walks the slice sequentially and
// relies on TryRole + TryState to decide what runs.
//
// Nesting: nested try: blocks are NOT supported in v1 (issue #67).
// Spec-23 §2 doesn't pin down the propagation semantics when an
// inner try fails and an outer try would also catch — every plausible
// answer is a forward-incompatible wedge. Mirrors spec-30's rejection
// of nested transactions. The "swallow a single inner step's failure"
// pattern is covered by `continue_on_error: true` on the leaf step.
func (p *Planner) expandTry(step config.Step, ctx *ExpansionContext, plan *Plan, _ int) error {
	if err := checkTryChildrenNoNesting(step); err != nil {
		return err
	}
	parent, err := p.compilePlanStep(step, ctx, nil)
	if err != nil {
		return fmt.Errorf("try %q: compile parent: %w", step.Name, err)
	}
	// Keep Try / Catch / Finally populated on the parent's plan entry
	// so plan readers (and the MCP tool surfacing the compound shape)
	// have the original structure available. Children also expand as
	// siblings so the executor can walk them as a flat sequence.
	tryChildren := parent.Try
	catchChildren := parent.Catch
	finallyChildren := parent.Finally
	plan.Steps = append(plan.Steps, parent)

	parentID := parent.ID
	if err := p.expandTryBranch(tryChildren, parentID, "try", step.Name, ctx, plan); err != nil {
		return err
	}
	if err := p.expandTryBranch(catchChildren, parentID, "catch", step.Name, ctx, plan); err != nil {
		return err
	}
	if err := p.expandTryBranch(finallyChildren, parentID, "finally", step.Name, ctx, plan); err != nil {
		return err
	}
	return nil
}

func (p *Planner) expandTryBranch(children []config.Step, parentID, role, parentName string, ctx *ExpansionContext, plan *Plan) error {
	for ci := range children {
		child := children[ci]
		child.TryParent = parentID
		child.TryRole = role
		if err := p.expandStep(child, ctx, plan, 0); err != nil {
			return fmt.Errorf("try %q: expand %s child %d: %w", parentName, role, ci, err)
		}
	}
	return nil
}

// checkTryChildrenNoNesting walks the try / catch / finally branches
// of a compound step and rejects any child that itself declares a
// try: block. Issue #67: without this, nested try children fell
// through to expandStep with no compound-handling, the dispatch saw
// `action: ""`, and the user got an opaque "no handler registered
// for action type: unknown" with no hint at the actual cause.
//
// The hint at continue_on_error: routes most "I just want to swallow
// this inner failure" cases — the common reason people reach for
// nested try.
func checkTryChildrenNoNesting(step config.Step) error {
	branches := []struct {
		name     string
		children []config.Step
	}{
		{"try", step.Try},
		{"catch", step.Catch},
		{"finally", step.Finally},
	}
	for _, b := range branches {
		for i, child := range b.children {
			if len(child.Try) > 0 {
				name := step.Name
				if name == "" {
					name = "<unnamed>"
				}
				return fmt.Errorf(
					"try block %q: %s child %d (%s): nested try: blocks are not supported in v1.\n"+
						"  Hint: to swallow a single inner step's failure, set `continue_on_error: true` on that step instead of wrapping it in another try:.",
					name, b.name, i, displayName(child),
				)
			}
		}
	}
	return nil
}
