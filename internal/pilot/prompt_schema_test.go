package pilot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestBuildSchemaChunk_RealSchema pins the rendered shape against the
// real embedded schema.json. Structural assertions only — not a golden
// snapshot — because the contents are derived from a file that legitimately
// changes when new actions land. The point is that the *shape* of the
// rendered chunk stays stable.
func TestBuildSchemaChunk_RealSchema(t *testing.T) {
	chunk, err := BuildSchemaChunk()
	if err != nil {
		t.Fatalf("BuildSchemaChunk: %v", err)
	}

	// Top-level structure: preamble + ACTIONS section + UNIVERSAL section.
	mustContain(t, chunk, "A Mooncake config is a YAML array of steps.")
	mustContain(t, chunk, "ACTIONS (grouped by category):")
	mustContain(t, chunk, "UNIVERSAL STEP FIELDS:")

	// Spot-check a stable set of actions that should always be in schema.
	// If any of these are renamed or removed, this test fails as a heads-up
	// that the prompt vocabulary is changing meaningfully.
	for _, name := range []string{
		"file.write",
		"cmd",
		"shell",
		"assert",
		"pkg",
		"os.service",
		"repo.search",
	} {
		// Each action should appear as a bullet line: "  - <name>".
		needle := "  - " + name + " "
		if !strings.Contains(chunk, needle) && !strings.Contains(chunk, "  - "+name+"\n") {
			t.Errorf("expected action %q to appear as a bullet in chunk", name)
		}
	}

	// Spot-check universal step fields are present.
	for _, name := range []string{"when", "as", "tags", "for_each"} {
		needle := "  - " + name + " "
		if !strings.Contains(chunk, needle) {
			t.Errorf("expected universal field %q in chunk", name)
		}
	}

	// Dead duplicates from the schema (defs without a step.properties
	// reference, e.g., cmd_action, shell_action) must not surface.
	for _, name := range []string{"cmd_action", "shell_action"} {
		if strings.Contains(chunk, "  - "+name) {
			t.Errorf("internal duplicate %q should not appear in prompt vocabulary", name)
		}
	}
}

// TestBuildSchemaChunk_AllStepActionsAppear is the load-bearing DoD test:
// every action wired into the step definition surfaces in the prompt,
// keyed off the real schema. If a new action is added to schema.json and
// referenced from step, this test passes automatically — no edit to
// prompt*.go required.
func TestBuildSchemaChunk_AllStepActionsAppear(t *testing.T) {
	var root struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Ref string `json:"$ref"`
			} `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(config.SchemaJSON(), &root); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	step, ok := root.Definitions["step"]
	if !ok {
		t.Fatalf("schema missing step definition")
	}

	chunk, err := BuildSchemaChunk()
	if err != nil {
		t.Fatalf("BuildSchemaChunk: %v", err)
	}

	for propName, prop := range step.Properties {
		if prop.Ref == "" {
			continue // universal modifier, checked separately
		}
		needle := "  - " + propName
		if !strings.Contains(chunk, needle) {
			t.Errorf("action %q (ref %s) wired into step.properties but missing from chunk",
				propName, prop.Ref)
		}
	}
}

// TestBuildSchemaChunkFromJSON_NewActionAppears is the explicit DoD test
// from spec-67 §16: "Adding a new in-tree action shows up in pilot's
// prompt vocabulary the same release. No source edit needed."
//
// We feed in a fake schema with one bespoke action and verify it surfaces.
func TestBuildSchemaChunkFromJSON_NewActionAppears(t *testing.T) {
	fakeSchema := []byte(`{
	  "definitions": {
	    "step": {
	      "type": "object",
	      "properties": {
	        "fictional.bake_cake": {
	          "description": "Bake a delicious cake",
	          "$ref": "#/definitions/fictional.bake_cake"
	        },
	        "when": {
	          "type": "string",
	          "description": "Conditional expression"
	        }
	      }
	    },
	    "fictional.bake_cake": {
	      "type": "object",
	      "description": "Bake a delicious cake",
	      "x-category": "dessert",
	      "properties": {
	        "flavor": {"type": "string", "description": "cake flavor"},
	        "layers": {"type": "integer"}
	      },
	      "required": ["flavor"]
	    }
	  }
	}`)
	chunk, err := buildSchemaChunkFromJSON(fakeSchema)
	if err != nil {
		t.Fatalf("buildSchemaChunkFromJSON: %v", err)
	}

	// New action surfaced.
	mustContain(t, chunk, "- fictional.bake_cake — Bake a delicious cake")
	// Category surfaced.
	mustContain(t, chunk, "[dessert]")
	// Required + typed fields surfaced.
	mustContain(t, chunk, "required: flavor (string)")
	// Optional field surfaced.
	mustContain(t, chunk, "optional: layers")
	// Universal modifier surfaced.
	mustContain(t, chunk, "- when (string): Conditional expression")
}

func TestBuildSchemaChunkFromJSON_MalformedSchema(t *testing.T) {
	if _, err := buildSchemaChunkFromJSON([]byte("not json")); err == nil {
		t.Fatal("expected error on malformed schema")
	}
	if _, err := buildSchemaChunkFromJSON([]byte(`{"definitions":{}}`)); err == nil {
		t.Fatal("expected error when step definition missing")
	}
}

// TestBuildPrompt_UsesDynamicChunk verifies the wiring: BuildPrompt's
// systemPrompt return value contains schema-derived content (proves
// the const→function refactor stuck).
func TestBuildPrompt_UsesDynamicChunk(t *testing.T) {
	systemPrompt, _, err := BuildPrompt(PlanInput{
		Goal:     "test goal",
		Snapshot: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	// Sentinel from BuildSchemaChunk's rendered output.
	mustContain(t, systemPrompt, "ACTIONS (grouped by category):")
	mustContain(t, systemPrompt, "UNIVERSAL STEP FIELDS:")
	// Sentinel from static preamble.
	mustContain(t, systemPrompt, "You are a Mooncake agent planner.")
	// Sentinel from static constraints.
	mustContain(t, systemPrompt, "All file paths must be absolute or relative to repo root")
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("missing %q in:\n%s", needle, haystack)
	}
}
