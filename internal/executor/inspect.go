package executor

import (
	"context"
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
)

// InspectPlan runs the plan in non-mutating ModePlan and returns per-step
// inspections. Each inspection reports whether applying the step would
// change state, the handler-supplied reason, and whether the step is
// checkable at all (shell steps return Checkable=false).
//
// This is the primitive that powers `mooncake plan` after Spec 16: the
// plan command builds the static plan via planner.BuildPlan, then calls
// InspectPlan to fill in the per-step state predictions.
//
// Implementation: subscribes a collector to a fresh SyncPublisher,
// dispatches the plan through the standard executor in check mode
// (which routes Runner handlers via dispatchRunner and legacy handlers
// via dispatchCheck — both emit EventStepChecked), then returns the
// collected results.
func InspectPlan(p *plan.Plan, sudoPass string, log logger.Logger) ([]plan.StepInspection, error) {
	return InspectPlanWithRegistry(p, sudoPass, log, nil)
}

// InspectPlanWithRegistry is InspectPlan with an explicit action
// registry threaded into check-mode dispatch. Pass a consumer-owned
// registry so a plan built on custom typed actions resolves those
// handlers' Differ/Coster output during the preview; nil falls back to
// the process-wide global (identical to InspectPlan). This is the
// dry-run primitive the public SDK's mooncake.Plan goes through so an
// external consumer gets diffs/costs for its own actions without
// mutating the global registry.
func InspectPlanWithRegistry(p *plan.Plan, sudoPass string, log logger.Logger, registry *actions.Registry) ([]plan.StepInspection, error) {
	collector := &inspectionCollector{
		byStepID: make(map[string]plan.StepInspection),
	}

	pub := events.NewSyncPublisher()
	pub.Subscribe(collector)
	defer pub.Close()

	// InspectPlan is plan-mode (no mutations, no blocking handlers),
	// so context.Background() is sufficient — there's nothing for a
	// cancellation to interrupt that wouldn't already complete in ms.
	// Add a ctx parameter only if a future use case needs deadlines on
	// inspection.
	// keepGoing: an inspection error belongs to ONE step, so it must not
	// blind the operator to the other 180. A plan is a preview — every
	// step that can be predicted should be, and the ones that can't say
	// why, in place, via the EventStepFailed branch in the collector.
	//
	// This is what made `mooncake plan` unusable on a fresh machine: a
	// step needing sudo with no password configured, or a binary that an
	// earlier step in the same plan installs, aborted the whole preview.
	err := executePlanWithCapture(context.Background(), p, sudoPass, actions.ModePlan, log, pub, nil, nil, registry, true)
	var deferred *DeferredFailuresError
	if err != nil && !errors.As(err, &deferred) {
		// A genuine setup error (bad plan, unusable renderer) still
		// aborts — that's not a per-step condition.
		return nil, err
	}
	return collector.collect(p), nil
}

// inspectionCollector subscribes to step events emitted during a
// check-mode run and assembles a per-step inspection slice in plan
// order. EventStepChecked is emitted by dispatchRunner (Spec 16
// handlers) and dispatchCheck (legacy Checker handlers) and the
// check-mode bypass in ExecuteStep for unknown actions.
//
// EventStepSkipped is also captured so the resulting slice reflects
// when/tag filtering decisions made at plan time.
type inspectionCollector struct {
	byStepID map[string]plan.StepInspection
}

func (c *inspectionCollector) OnEvent(e events.Event) {
	switch e.Type {
	case events.EventStepChecked:
		d, ok := e.Data.(events.StepCheckedData)
		if !ok {
			return
		}
		// Diff is carried as `any` through the event (events package
		// can't import actions without a cycle); cast back to the
		// typed *actions.Diff here. The cast fails silently for
		// non-Differ handlers — d.Diff is nil and so is diff.
		diff, _ := d.Diff.(*actions.Diff)
		// Cost: same any-typing rationale as Diff above. Nil for
		// non-Coster handlers.
		cost, _ := d.Cost.(*actions.CostEstimate)
		c.byStepID[d.StepID] = plan.StepInspection{
			StepID:      d.StepID,
			ActionType:  d.Action,
			WouldChange: d.WouldChange,
			Checkable:   d.Checkable,
			Reason:      d.Reason,
			Detail:      d.Detail,
			Diff:        diff,
			Cost:        cost,
		}
	case events.EventStepFailed:
		// Reached only in plan mode via the keepGoing path above: the
		// step could not be inspected (needs sudo, needs a binary an
		// earlier step installs, handler errored). Not a plan failure —
		// a plan-shaped answer of "can't tell you about this one".
		d, ok := e.Data.(events.StepFailedData)
		if !ok {
			return
		}
		c.byStepID[d.StepID] = plan.StepInspection{
			StepID:     d.StepID,
			ActionType: d.Action,
			Checkable:  false,
			Reason:     fmt.Sprintf("not checkable: %s", d.ErrorMessage),
		}
	case events.EventStepSkipped:
		d, ok := e.Data.(events.StepSkippedData)
		if !ok {
			return
		}
		c.byStepID[d.StepID] = plan.StepInspection{
			StepID:  d.StepID,
			Skipped: true,
			Reason:  d.Reason,
		}
	}
}

func (c *inspectionCollector) Close() {}

// collect returns inspections in plan-step order. Steps without a
// matching event (which shouldn't happen for well-formed plans, but
// could for unknown step types) get a synthetic "not inspected" entry.
func (c *inspectionCollector) collect(p *plan.Plan) []plan.StepInspection {
	out := make([]plan.StepInspection, 0, len(p.Steps))
	for _, step := range p.Steps {
		if ins, ok := c.byStepID[step.ID]; ok {
			out = append(out, ins)
			continue
		}
		out = append(out, plan.StepInspection{
			StepID:    step.ID,
			Checkable: false,
			Reason:    "not inspected",
		})
	}
	return out
}
