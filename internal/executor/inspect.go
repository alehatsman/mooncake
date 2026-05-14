package executor

import (
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
	collector := &inspectionCollector{
		byStepID: make(map[string]plan.StepInspection),
	}

	pub := events.NewSyncPublisher()
	pub.Subscribe(collector)
	defer pub.Close()

	if err := ExecutePlan(p, sudoPass, actions.ModePlan, log, pub); err != nil {
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
		c.byStepID[d.StepID] = plan.StepInspection{
			StepID:      d.StepID,
			ActionType:  d.Action,
			WouldChange: d.WouldChange,
			Checkable:   d.Checkable,
			Reason:      d.Reason,
			Detail:      d.Detail,
			Diff:        diff,
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
