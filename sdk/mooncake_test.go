package mooncake_test

import (
	"strings"
	"testing"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// greetHandler is a custom typed action a consumer might register. It carries
// no config-struct binding (the generic carrier that lets such an action run
// from a real plan is the #111 follow-up); this test exercises the #105
// unlock: a registered handler surfaces in the planner's vocabulary and lives
// in an isolated registry.
type greetHandler struct{}

func (greetHandler) Metadata() mooncake.ActionMetadata {
	return mooncake.ActionMetadata{
		Name:        "demo.greet",
		Description: "send a greeting (custom framework action)",
	}
}

func (greetHandler) Validate(*mooncake.Step) error { return nil }

func (greetHandler) Run(_ mooncake.Context, _ *mooncake.Step) (mooncake.Result, error) {
	return mooncake.NewResult(), nil
}

// TestCustomActionSurfacesInSchemaChunk is the consumer-facing acceptance for
// #105: build a registry from the built-ins, register a custom typed handler,
// and confirm it appears in the planner's action vocabulary derived from that
// live registry — with no schema.json edit and no internal/ import.
func TestCustomActionSurfacesInSchemaChunk(t *testing.T) {
	reg := mooncake.DefaultRegistry()
	if err := reg.Register(greetHandler{}); err != nil {
		t.Fatalf("register custom handler: %v", err)
	}

	chunk, err := mooncake.BuildSchemaChunkForRegistry(reg)
	if err != nil {
		t.Fatalf("build schema chunk: %v", err)
	}

	if !strings.Contains(chunk, "demo.greet") {
		t.Errorf("custom action missing from schema chunk:\n%s", chunk)
	}
	if !strings.Contains(chunk, "send a greeting") {
		t.Errorf("custom action description missing from schema chunk:\n%s", chunk)
	}
	// The built-ins must still be present (the registry was cloned, not reset).
	if !strings.Contains(chunk, "shell") {
		t.Errorf("built-in actions missing from cloned registry's schema chunk")
	}
}

// TestDefaultRegistryIsIsolated guards the clone semantics: registering into a
// DefaultRegistry must not leak into the global, so two consumers can hold
// divergent vocabularies in the same process.
func TestDefaultRegistryIsIsolated(t *testing.T) {
	reg := mooncake.DefaultRegistry()
	if !reg.Has("shell") {
		t.Fatal("DefaultRegistry must be seeded with the built-ins")
	}
	if err := reg.Register(greetHandler{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if mooncake.GlobalRegistry().Has("demo.greet") {
		t.Error("registering into a DefaultRegistry leaked into the global registry")
	}
	if !mooncake.DefaultRegistry().Has("shell") || mooncake.DefaultRegistry().Has("demo.greet") {
		t.Error("a fresh DefaultRegistry must have built-ins but not the prior clone's custom action")
	}
}
