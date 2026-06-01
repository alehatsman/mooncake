package agent

import (
	"context"
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
)

// streamingStub implements both llm.Client and llm.StreamingClient so
// streamPlan takes the streaming branch.
type streamingStub struct {
	deltas []events.PlannerDeltaData // text+kind to replay (Iteration ignored)
	final  string
}

func (s *streamingStub) GeneratePlan(_ context.Context, _, _, _ string) (string, error) {
	return s.final, nil
}

func (s *streamingStub) GeneratePlanStream(_ context.Context, _, _, _ string, onDelta func(text, kind string)) (string, error) {
	for _, d := range s.deltas {
		onDelta(d.Text, d.Kind)
	}
	return s.final, nil
}

// recordingSub captures every published event for assertions.
type recordingSub struct{ events []events.Event }

func (r *recordingSub) OnEvent(e events.Event) { r.events = append(r.events, e) }
func (r *recordingSub) Close()                 {}

func TestStreamPlan_StreamingProvider(t *testing.T) {
	pub := events.NewSyncPublisher()
	rec := &recordingSub{}
	pub.Subscribe(rec)

	stub := &streamingStub{
		deltas: []events.PlannerDeltaData{
			{Text: "thinking...", Kind: "thinking"},
			{Text: "- shell:\n", Kind: "text"},
			{Text: "    cmd: echo hi", Kind: "text"},
			{Text: "", Kind: "text"}, // empty deltas must be dropped
		},
		final: "- shell:\n    cmd: echo hi",
	}

	got, err := streamPlan(context.Background(), stub, pub, 3,
		RunOptions{Provider: "anthropic-cli", Model: "m"}, "sys", "user")
	pub.Close()
	if err != nil {
		t.Fatalf("streamPlan error: %v", err)
	}
	if got != stub.final {
		t.Errorf("plan = %q, want %q", got, stub.final)
	}

	// First event is the plan.generating bracket carrying the iteration.
	if len(rec.events) == 0 || rec.events[0].Type != events.EventPlanGenerating {
		t.Fatalf("first event = %v, want plan.generating", rec.events)
	}
	if d, ok := rec.events[0].Data.(events.PlanGeneratingData); !ok || d.Iteration != 3 || d.Provider != "anthropic-cli" {
		t.Errorf("plan.generating data = %+v, want iteration 3 / provider anthropic-cli", rec.events[0].Data)
	}

	// Remaining events are planner.delta, one per non-empty delta, in order,
	// stamped with the iteration. The empty delta must not produce an event.
	var deltas []events.PlannerDeltaData
	for _, e := range rec.events[1:] {
		if e.Type != events.EventPlannerDelta {
			t.Fatalf("unexpected event type %q after bracket", e.Type)
		}
		d := e.Data.(events.PlannerDeltaData)
		if d.Iteration != 3 {
			t.Errorf("delta iteration = %d, want 3", d.Iteration)
		}
		deltas = append(deltas, d)
	}
	want := []events.PlannerDeltaData{
		{Iteration: 3, Text: "thinking...", Kind: "thinking"},
		{Iteration: 3, Text: "- shell:\n", Kind: "text"},
		{Iteration: 3, Text: "    cmd: echo hi", Kind: "text"},
	}
	if len(deltas) != len(want) {
		t.Fatalf("got %d planner.delta events, want %d", len(deltas), len(want))
	}
	for i := range want {
		if deltas[i] != want[i] {
			t.Errorf("delta[%d] = %+v, want %+v", i, deltas[i], want[i])
		}
	}
}

func TestStreamPlan_BufferedFallbackSyntheticDelta(t *testing.T) {
	pub := events.NewSyncPublisher()
	rec := &recordingSub{}
	pub.Subscribe(rec)

	// stubLLMClient (loop_test.go) implements only GeneratePlan, so streamPlan
	// takes the fallback branch and synthesizes one full-text delta.
	stub := &stubLLMClient{plans: []string{"FULL PLAN BODY"}}

	got, err := streamPlan(context.Background(), stub, pub, 1, RunOptions{}, "sys", "user")
	pub.Close()
	if err != nil {
		t.Fatalf("streamPlan error: %v", err)
	}
	if got != "FULL PLAN BODY" {
		t.Errorf("plan = %q, want %q", got, "FULL PLAN BODY")
	}

	if len(rec.events) != 2 {
		t.Fatalf("got %d events, want 2 (plan.generating + one synthetic delta)", len(rec.events))
	}
	if rec.events[0].Type != events.EventPlanGenerating {
		t.Errorf("event[0] = %q, want plan.generating", rec.events[0].Type)
	}
	d, ok := rec.events[1].Data.(events.PlannerDeltaData)
	if rec.events[1].Type != events.EventPlannerDelta || !ok {
		t.Fatalf("event[1] = %v, want planner.delta", rec.events[1])
	}
	if d.Text != "FULL PLAN BODY" || d.Kind != "text" || d.Iteration != 1 {
		t.Errorf("synthetic delta = %+v, want full text / kind text / iteration 1", d)
	}
}
