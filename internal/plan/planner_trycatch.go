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
func (p *Planner) expandTry(step config.Step, ctx *ExpansionContext, plan *Plan, _ int) error {
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
