package plan

// planner_transaction.go implements the spec-30 §93 transaction-block
// expansion. The parser deposits a Step with non-empty Transaction
// children + optional OnRollback + AllowIrreversible flag; this file
// turns that into plan-level steps with TxnParent linkage and runs
// the plan-time reversibility check.
//
// What this file does NOT do: any execution semantics. The executor
// side (forward apply + LIFO rollback) ships in the follow-up PR
// when spec-22 phase 5 covers enough handlers to drive a real demo.
// This file lays the rails so the executor work is straightforward.

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// expandTransaction emits one parent plan step (no action; carries
// the transaction's metadata) followed by each child as a sibling
// step with TxnParent = parent.ID. Reversibility check (spec-30 §76):
// every child must have a registered handler that implements
// actions.Reverser, unless step.AllowIrreversible is true.
//
// Refuses to plan if any child carries its own Transaction
// (nested transactions need different roll-back semantics; deferred to
// a future spec).
func (p *Planner) expandTransaction(step config.Step, ctx *ExpansionContext, plan *Plan, _ int) error {
	// Reversibility check, BEFORE we mutate the plan. Building a partial
	// plan when we'll bail anyway is messy and would surface a confusing
	// "successful with errors" state.
	if !step.AllowIrreversible {
		if err := checkChildrenReversible(step.Transaction); err != nil {
			return fmt.Errorf("transaction %q: %w (set allow_irreversible: true to override)", step.Name, err)
		}
	}

	// Materialize a transaction-parent plan step. Carries Transaction,
	// OnRollback, and AllowIrreversible verbatim so the executor (PR B)
	// has the full shape on hand and can re-validate at apply time. The
	// parent has no action discriminator — config.Step.Validate already
	// allowed that branch.
	parent, err := p.compilePlanStep(step, ctx, nil)
	if err != nil {
		return fmt.Errorf("transaction %q: compile parent: %w", step.Name, err)
	}
	// Clear the children/on_rollback off the parent's plan entry: they
	// expand as siblings, exactly like on_change. Linkage survives via
	// the children's TxnParent.
	txnChildren := parent.Transaction
	rollbackChildren := parent.OnRollback
	parent.Transaction = nil
	parent.OnRollback = nil
	plan.Steps = append(plan.Steps, parent)

	parentID := parent.ID
	for ci := range txnChildren {
		child := txnChildren[ci]
		child.TxnParent = parentID
		if err := p.expandStep(child, ctx, plan, 0); err != nil {
			return fmt.Errorf("transaction %q: expand child %d: %w", step.Name, ci, err)
		}
	}
	// on_rollback children sit at the end. They're tagged with TxnParent
	// the same way; the executor (PR B) tells them apart from regular
	// children via a separate marker. For PR A we just emit them so the
	// shape round-trips through plan serialization.
	for ci := range rollbackChildren {
		child := rollbackChildren[ci]
		child.TxnParent = parentID
		// Distinguish rollback steps from transaction body steps via a
		// name prefix. PR B will replace this with a typed flag once the
		// executor has a shape to consume.
		if child.Name == "" {
			child.Name = "on_rollback"
		}
		child.Name = "rollback: " + child.Name
		if err := p.expandStep(child, ctx, plan, 0); err != nil {
			return fmt.Errorf("transaction %q: expand on_rollback child %d: %w", step.Name, ci, err)
		}
	}
	return nil
}

// checkChildrenReversible walks each child's action discriminator,
// looks up the registered handler in the global actions registry, and
// asserts it implements actions.Reverser. Returns a descriptive error
// at the first non-reverser child found so the operator knows which
// step to either replace or wrap in allow_irreversible.
//
// A child that is itself a Transaction is currently rejected — nested
// transactions are out of scope for v1 (per spec-30 §202).
//
// A child whose action is unknown (no registered handler) is also
// rejected; the schema validator should have caught it earlier but
// the planner is the final arbiter.
func checkChildrenReversible(children []config.Step) error {
	for i, child := range children {
		if len(child.Transaction) > 0 {
			return fmt.Errorf("child %d (%s): nested transactions are not supported in v1", i, displayName(child))
		}
		actionType := child.DetermineActionType()
		if actionType == "" {
			return fmt.Errorf("child %d (%s): no action set", i, displayName(child))
		}
		handler, ok := actions.Get(actionType)
		if !ok {
			return fmt.Errorf("child %d (%s): unknown action %q", i, displayName(child), actionType)
		}
		if _, ok := handler.(actions.Reverser); !ok {
			return fmt.Errorf("child %d (%s): action %q does not implement Reverser — cannot roll back automatically", i, displayName(child), actionType)
		}
	}
	return nil
}

// displayName returns a human-readable label for a child step in
// error messages. Prefers Name, falls back to the action discriminator.
func displayName(s config.Step) string {
	if s.Name != "" {
		return s.Name
	}
	if t := s.DetermineActionType(); t != "" {
		return t
	}
	return "<unnamed>"
}
