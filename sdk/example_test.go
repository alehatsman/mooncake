package mooncake_test

import (
	"fmt"
	"strings"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// issueHandler is a custom typed action a consumer registers into the kernel.
// It implements the Handler ABI (Metadata/Validate/Run); reading its
// parameters off step.With and declaring Permissions/Cost/Reverse would
// spread the typed kernel guarantees to it. A plan reaches it via the generic
// carrier: {"action": "demo.issue", "with": {...}}.
type issueHandler struct{}

func (issueHandler) Metadata() mooncake.ActionMetadata {
	return mooncake.ActionMetadata{
		Name:        "demo.issue",
		Description: "open a tracking issue (custom framework action)",
	}
}

func (issueHandler) Validate(*mooncake.Step) error { return nil }

func (issueHandler) Run(_ mooncake.Context, step *mooncake.Step) (mooncake.Result, error) {
	title, _ := step.With["title"].(string)
	r := mooncake.NewResult()
	r.Stdout = "opened: " + title
	r.Changed = true
	return r, nil
}

// Example shows the agent-framework entry point: seed a registry with the
// built-ins, register a custom typed action, and confirm it joins the
// planner's vocabulary — the surface a consumer compiles its own agent on.
// To run it, hand the registry (and optionally a custom LLMClient backend) to
// mooncake.RunLoop via RunOptions{Registry: reg, LLMClient: ...}.
func Example() {
	// Start from the built-ins, then add a custom action. Equivalent to
	// mooncake.DefaultRegistry(); shown the explicit way here.
	reg := mooncake.NewRegistry()
	if err := mooncake.RegisterBuiltins(reg); err != nil {
		panic(err)
	}
	if err := reg.Register(issueHandler{}); err != nil {
		panic(err)
	}

	// The custom action surfaces in the planner's vocabulary derived from
	// this live registry — no schema.json edit, no prompt change.
	chunk, err := mooncake.BuildSchemaChunkForRegistry(reg)
	if err != nil {
		panic(err)
	}

	fmt.Println("built-ins present:", reg.Has("shell"))
	fmt.Println("custom action present:", reg.Has("demo.issue"))
	fmt.Println("custom action in planner vocabulary:", strings.Contains(chunk, "demo.issue"))
	// Output:
	// built-ins present: true
	// custom action present: true
	// custom action in planner vocabulary: true
}
