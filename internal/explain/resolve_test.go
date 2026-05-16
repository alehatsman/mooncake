package explain

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	_ "github.com/alehatsman/mooncake/internal/register" // ensure handlers register
)

func TestResolve_EmptyNoun(t *testing.T) {
	r := Resolve("  ", Options{})
	if r.Kind != KindNotFound {
		t.Fatalf("kind = %q, want %q", r.Kind, KindNotFound)
	}
	if r.NotFound == nil || !strings.Contains(r.NotFound.Reason, "empty") {
		t.Fatalf("not_found payload = %+v, want reason mentioning empty", r.NotFound)
	}
}

func TestResolve_EveryRegisteredActionResolves(t *testing.T) {
	all := actions.List()
	if len(all) == 0 {
		t.Skip("no actions registered; register import didn't initialize")
	}
	for _, m := range all {
		t.Run(m.Name, func(t *testing.T) {
			r := Resolve(m.Name, Options{ExamplesRoot: "."}) // skip example scan
			if r.Kind != KindAction {
				t.Fatalf("kind = %q, want %q", r.Kind, KindAction)
			}
			if r.Action == nil {
				t.Fatal("Action payload is nil")
			}
			if r.Action.Name != m.Name {
				t.Errorf("Action.Name = %q, want %q", r.Action.Name, m.Name)
			}
			if r.Action.Metadata.Description == "" {
				t.Error("Action.Metadata.Description is empty")
			}
			// DiffShape and ReverseShape always carry a note/caveat.
			if r.Action.DiffShape.Note == "" {
				t.Error("DiffShape.Note is empty")
			}
			if r.Action.ReverseShape.Caveat == "" {
				t.Error("ReverseShape.Caveat is empty")
			}
		})
	}
}

func TestResolve_UnknownActionNotFoundWithCandidates(t *testing.T) {
	r := Resolve("shellish", Options{ExamplesRoot: "."})
	if r.Kind != KindNotFound {
		t.Fatalf("kind = %q, want %q", r.Kind, KindNotFound)
	}
	if r.NotFound == nil {
		t.Fatal("NotFound payload is nil")
	}
	// "shell" should be a near-match candidate.
	var sawShell bool
	for _, c := range r.NotFound.Candidates {
		if c.ID == "shell" && c.Kind == KindAction {
			sawShell = true
		}
	}
	if !sawShell {
		t.Errorf("candidates = %+v, want shell among suggestions", r.NotFound.Candidates)
	}
}

func TestResolve_UnrecognisedNounShape(t *testing.T) {
	r := Resolve("???", Options{})
	if r.Kind != KindNotFound {
		t.Fatalf("kind = %q, want %q", r.Kind, KindNotFound)
	}
	if r.NotFound.Reason != "unrecognised noun shape" {
		t.Errorf("reason = %q, want unrecognised noun shape", r.NotFound.Reason)
	}
}

func TestResolve_RunOpResourceAreWave2NotFound(t *testing.T) {
	cases := []struct {
		noun string
		kind Kind
	}{
		{"r/01HV5K", KindRun},
		{"op/01HV5K", KindOp},
		{"file:/etc/hosts", KindResource},
	}
	for _, tc := range cases {
		t.Run(tc.noun, func(t *testing.T) {
			r := Resolve(tc.noun, Options{})
			if r.Kind != KindNotFound {
				t.Fatalf("kind = %q, want %q", r.Kind, KindNotFound)
			}
			if !strings.Contains(r.NotFound.Reason, "wave 2") {
				t.Errorf("reason = %q, want mention of wave 2", r.NotFound.Reason)
			}
		})
	}
}

func TestDetectKind(t *testing.T) {
	cases := []struct {
		noun string
		want Kind
	}{
		{"shell", KindAction},
		{"pkg", KindAction},
		{"pkg.install", KindAction},
		{"r/01HV5K", KindRun},
		{"op/01HV5K", KindOp},
		{"file:/etc/hosts", KindResource},
		{"pkg:apt/curl", KindResource},
		{"http://example.com/x", KindNotFound},
		{"???", KindNotFound},
		{"", KindNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.noun, func(t *testing.T) {
			got := detectKind(tc.noun)
			if got != tc.want {
				t.Errorf("detectKind(%q) = %q, want %q", tc.noun, got, tc.want)
			}
		})
	}
}

func TestExtractSchemaSlice_FromOverride(t *testing.T) {
	raw := []byte(`{
		"definitions": {
			"widget": {
				"required": ["name"],
				"properties": {
					"name":  { "type": "string", "description": "Widget name" },
					"count": { "type": "integer", "default": 1 }
				}
			}
		}
	}`)

	slice := extractSchemaSlice("widget", raw)
	if slice == nil {
		t.Fatal("extractSchemaSlice returned nil")
	}
	if len(slice.Required) != 1 || slice.Required[0] != "name" {
		t.Errorf("Required = %v, want [name]", slice.Required)
	}
	if slice.Properties["name"].Type != "string" {
		t.Errorf("name.Type = %q, want string", slice.Properties["name"].Type)
	}
	if slice.Properties["name"].Description != "Widget name" {
		t.Errorf("name.Description = %q", slice.Properties["name"].Description)
	}
	if slice.Properties["count"].Default == nil {
		t.Errorf("count.Default = nil, want 1")
	}
}

func TestExtractSchemaSlice_UnknownNounReturnsNil(t *testing.T) {
	raw := []byte(`{"definitions": {"widget": {}}}`)
	if slice := extractSchemaSlice("frobnicate", raw); slice != nil {
		t.Errorf("expected nil for unknown noun, got %+v", slice)
	}
}

func TestExtractSchemaSlice_MalformedJSONReturnsNil(t *testing.T) {
	if slice := extractSchemaSlice("anything", []byte("{not json")); slice != nil {
		t.Errorf("expected nil for malformed JSON, got %+v", slice)
	}
}

func TestActionCandidates_RanksPrefixHighest(t *testing.T) {
	cands := actionCandidates("shel")
	if len(cands) == 0 {
		t.Fatal("no candidates returned")
	}
	if cands[0].ID != "shell" {
		t.Errorf("first candidate = %q, want shell", cands[0].ID)
	}
}

func TestLooksLikeActionVerb(t *testing.T) {
	cases := map[string]bool{
		"shell":       true,
		"pkg.install": true,
		"a_b-c.d_e":   true,
		"":            false,
		"has space":   false,
		"with/slash":  false,
		"r/01HV5K":    false,
	}
	for noun, want := range cases {
		got := looksLikeActionVerb(noun)
		if got != want {
			t.Errorf("looksLikeActionVerb(%q) = %v, want %v", noun, got, want)
		}
	}
}
