package mooncake_test

import (
	"fmt"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// greeterHandler is a toy custom action a consumer might author. It reads a
// `name` parameter off step.With, renders a greeting through the context's
// template renderer (so {{ }} expressions work exactly as in production),
// emits an observability event, and returns a Result. A consumer can
// unit-test this Run in complete isolation — no executor, no agent loop —
// importing only the public github.com/alehatsman/mooncake/sdk path.
type greeterHandler struct{}

func (greeterHandler) Metadata() mooncake.ActionMetadata {
	return mooncake.ActionMetadata{
		Name:        "demo.greeter",
		Description: "render a greeting (toy custom action)",
	}
}

func (greeterHandler) Validate(step *mooncake.Step) error {
	if _, ok := mooncake.WithString(step, "name"); !ok {
		return fmt.Errorf("demo.greeter: `name` is required and must be a string")
	}
	return nil
}

func (greeterHandler) Run(ctx mooncake.Context, step *mooncake.Step) (mooncake.Result, error) {
	name, _ := mooncake.WithString(step, "name")

	// Render through the real renderer so template syntax is exercised.
	rendered, err := ctx.Template().Render("Hello, {{ name }}!", map[string]any{"name": name})
	if err != nil {
		return nil, err
	}

	ctx.EventPublisher().Publish(mooncake.Event{Type: "demo.greeting.rendered"})

	r := mooncake.NewResult()
	r.Stdout = rendered
	r.Changed = true
	return r, nil
}

// ExampleNewTestContext shows a consumer unit-testing a custom Handler's Run
// in isolation: build a Context with seeded vars, call Run, then assert on the
// returned Result and the events the handler emitted — all through the public
// SDK surface alone.
func ExampleNewTestContext() {
	h := greeterHandler{}
	step := &mooncake.Step{With: map[string]any{"name": "Ada"}}

	// Validate round-trips a well-formed step.
	if err := h.Validate(step); err != nil {
		panic(err)
	}

	// Drive Run with a test context. The default publisher captures events.
	ctx := mooncake.NewTestContext(mooncake.WithMode(mooncake.ModeApply))
	res, err := h.Run(ctx, step)
	if err != nil {
		panic(err)
	}

	// The Result is the concrete *ResultData the handler populated.
	rd := res.(*mooncake.ResultData)
	fmt.Println("stdout:", rd.Stdout)
	fmt.Println("changed:", rd.Changed)

	// Events the handler emitted are recoverable from the context.
	evts := mooncake.CapturedEvents(ctx)
	fmt.Println("events emitted:", len(evts))
	if len(evts) > 0 {
		fmt.Println("first event type:", evts[0].Type)
	}

	// The conformance helper checks the ABI contract in one call.
	if err := mooncake.AssertHandlerConformance(h, step, mooncake.NewResult()); err != nil {
		panic(err)
	}
	fmt.Println("conformance: ok")

	// Output:
	// stdout: Hello, Ada!
	// changed: true
	// events emitted: 1
	// first event type: demo.greeting.rendered
	// conformance: ok
}
