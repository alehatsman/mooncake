package executor

import (
	"sync"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/plan"
)

// RunCapture is an optional sink the executor populates during
// Start / ExecutePlan with the compiled plan and per-step outcomes.
// Built specifically for R1.1b's typed *KernelResult on
// internal/apply.Runner: the Runner installs a *RunCapture before
// Start, reads its contents after Start returns, and converts them
// into the kernel-surface KernelResult.
//
// Other callers of executor.Start pass nil — the executor's hot path
// is a no-op when Capture is unset.
//
// Concurrency: protected by an internal mutex. The executor runs steps
// sequentially today, but the mutex guards against future parallel
// execution and against concurrent reads by the holder of the pointer.
type RunCapture struct {
	mu    sync.Mutex
	plan  *plan.Plan
	steps []StepRecord
}

// StepRecord is a single per-step entry — the typed step the executor
// dispatched plus the executor.Result it produced. apply.StepResult
// is the public mirror.
type StepRecord struct {
	// Step is the typed step as it ran (after planner expansion). Carries
	// the action, ID, tags, transaction membership, and — critically —
	// the ReverseData snapshot captured pre-mutation (spec-22 phase 5).
	Step config.Step

	// Result is the executor.Result produced by the handler.
	// nil only if the step was filtered out at dispatch time before a
	// result could be assembled.
	Result *Result

	// Reverted is set true when a transaction rollback's Reverse()
	// pass undid this step's mutation. F054 / spec-30: rolled-back
	// steps stay in the record (the original action ran; the
	// inverse undid it) so `mooncake history` and `explain` can
	// show "this step ran but was rolled back". Mutated via
	// markStepReverted from transaction.go's rollback walk.
	Reverted bool
}

// setPlan records the compiled plan. Called once at the top of
// ExecutePlan before any step runs.
func (c *RunCapture) setPlan(p *plan.Plan) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.plan = p
}

// appendStep records one step's outcome. Called from the executor's
// step-completion site (where ec.CurrentResult is still live).
func (c *RunCapture) appendStep(step config.Step, result *Result) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Copy the result so the executor can safely nil out ec.CurrentResult
	// after this call (the caller does that next).
	var copyResult *Result
	if result != nil {
		r := *result
		copyResult = &r
	}
	c.steps = append(c.steps, StepRecord{Step: step, Result: copyResult})
}

// markStepReverted flips the Reverted flag on the most recent
// StepRecord whose Step.ID matches stepID. Called from
// transaction.go's rollback walk after a successful Reverse(); the
// record was already appended when the original body step ran. Most
// recent (search from the tail) covers the rare case of duplicate
// IDs inside a long-running daemon's accumulated capture — newer
// wins, matching the LIFO walk semantics.
//
// No-op if the step ID isn't found: rollback can outlive its
// originating capture (e.g. a fault-injection test that records
// without registering steps). The transaction state machine's own
// counters carry the authoritative rollback tally; this method
// just decorates the per-step record for `mooncake history`.
func (c *RunCapture) markStepReverted(stepID string) {
	if c == nil || stepID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.steps) - 1; i >= 0; i-- {
		if c.steps[i].Step.ID == stepID {
			c.steps[i].Reverted = true
			return
		}
	}
}

// Plan returns the compiled plan recorded during the run, or nil
// if the run never reached plan compilation.
func (c *RunCapture) Plan() *plan.Plan {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.plan
}

// Steps returns a snapshot of the per-step records in execution order.
// The returned slice is owned by the caller; subsequent appends to the
// capture will not affect it.
func (c *RunCapture) Steps() []StepRecord {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]StepRecord, len(c.steps))
	copy(out, c.steps)
	return out
}
