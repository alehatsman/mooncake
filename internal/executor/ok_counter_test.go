package executor_test

import (
	"context"
	"sync"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	_ "github.com/alehatsman/mooncake/internal/actions/print"
	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
)

// TestF6_OkCounter_InvariantOnApply pins the OK + Changed == Executed
// invariant for a mixed apply: one log step (no change) and one shell
// step (changed=true). Reads the run.completed event so we cover both
// the bump site (postExecuteSuccess) and the publish-side population
// of RunCompletedData.OkSteps.
func TestF6_OkCounter_InvariantOnApply(t *testing.T) {
	cap := newRunCompletedCapture()
	publisher := events.NewPublisher()
	publisher.Subscribe(cap)
	defer publisher.Close()

	p := &plan.Plan{
		RootFile:    "test.yml",
		InitialVars: map[string]interface{}{},
		Tags:        []string{},
		Steps: []config.Step{
			{Name: "noop log", ActionType: "log", Log: &config.PrintAction{Msg: "hello"}},
			{Name: "changing shell", ActionType: "shell", Shell: &config.ShellAction{Cmd: "true"}},
		},
	}

	err := executor.ExecutePlan(context.Background(), p, "", actions.ModeApply, logger.NewTestLogger(), publisher)
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}
	publisher.Flush()

	data := cap.last(t)
	if got, want := data.OkSteps+data.ChangedSteps, data.SuccessSteps; got != want {
		t.Errorf("invariant OkSteps+ChangedSteps == SuccessSteps broken: %d+%d != %d (data=%+v)",
			data.OkSteps, data.ChangedSteps, data.SuccessSteps, data)
	}
	if data.OkSteps == 0 {
		t.Errorf("expected OkSteps > 0 (log step completes without Changed); got data=%+v", data)
	}
	if data.ChangedSteps == 0 {
		t.Errorf("expected ChangedSteps > 0 (shell step flips Changed); got data=%+v", data)
	}
}

// TestF6_OkCounter_InvariantOnPlan pins the same invariant in plan
// mode where the bump site is the dispatch path, not postExecuteSuccess.
func TestF6_OkCounter_InvariantOnPlan(t *testing.T) {
	cap := newRunCompletedCapture()
	publisher := events.NewPublisher()
	publisher.Subscribe(cap)
	defer publisher.Close()

	p := &plan.Plan{
		RootFile:    "test.yml",
		InitialVars: map[string]interface{}{},
		Tags:        []string{},
		Steps: []config.Step{
			{Name: "noop log", ActionType: "log", Log: &config.PrintAction{Msg: "hi"}},
		},
	}

	err := executor.ExecutePlan(context.Background(), p, "", actions.ModePlan, logger.NewTestLogger(), publisher)
	if err != nil {
		t.Fatalf("ExecutePlan(plan-mode): %v", err)
	}
	publisher.Flush()

	data := cap.last(t)
	if got, want := data.OkSteps+data.ChangedSteps, data.SuccessSteps; got != want {
		t.Errorf("invariant OkSteps+ChangedSteps == SuccessSteps broken in plan mode: %d+%d != %d (data=%+v)",
			data.OkSteps, data.ChangedSteps, data.SuccessSteps, data)
	}
}

// runCompletedCapture is a test-only events.Subscriber that retains
// every run.completed event so individual tests can dig out the last one.
type runCompletedCapture struct {
	mu     sync.Mutex
	events []events.RunCompletedData
}

func newRunCompletedCapture() *runCompletedCapture {
	return &runCompletedCapture{}
}

func (c *runCompletedCapture) OnEvent(e events.Event) {
	if e.Type != events.EventRunCompleted {
		return
	}
	d, ok := e.Data.(events.RunCompletedData)
	if !ok {
		return
	}
	c.mu.Lock()
	c.events = append(c.events, d)
	c.mu.Unlock()
}

func (c *runCompletedCapture) Close() {}

func (c *runCompletedCapture) last(t *testing.T) events.RunCompletedData {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) == 0 {
		t.Fatalf("no run.completed event captured")
	}
	return c.events[len(c.events)-1]
}
