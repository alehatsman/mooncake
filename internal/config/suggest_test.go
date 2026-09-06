package config

import "testing"

// Unknown-field errors used to end at the bare field name, so a
// one-character typo cost a docs round-trip (#174).
func TestSuggestField_Typos(t *testing.T) {
	cases := []struct {
		unknown string
		want    string
	}{
		{"file.wrte", "file.write"},
		{"wehn", "when"},
		{"tagz", "tags"},
		{"nmae", "name"},
		{"tiemout", "timeout"},
		// Renamed fields are further than any edit distance can reach,
		// so they come from the explicit rename table.
		{"become", "as_user"},
		{"become_user", "as_user"},
		{"register", "as"},
		{"with_items", "for_each"},
		{"with_filetree", "for_each_file"},
		{"parameters", "props"},
		// Nothing close enough: better to say nothing than to guess.
		{"frobnicate", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.unknown, func(t *testing.T) {
			got := suggestField(tc.unknown, candidatesForType("config.Step"))
			if got != tc.want {
				t.Errorf("suggestField(%q) = %q, want %q", tc.unknown, got, tc.want)
			}
		})
	}
}

// A legal field is distance 0 from itself; the suggester must not be
// consulted for one, but if it is, it must not invent a different answer.
func TestSuggestField_ExactMatchIsItself(t *testing.T) {
	for _, name := range []string{"when", "tags", "as_user", "file.write"} {
		if got := suggestField(name, candidatesForType("config.Step")); got != name {
			t.Errorf("suggestField(%q) = %q, want itself", name, got)
		}
	}
}

// candidatesForType routes to the right struct's vocabulary. `modules`
// is a RunConfig field and has no Step equivalent.
func TestCandidatesForType_RoutesByStruct(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}
	if !has(candidatesForType("config.RunConfig"), "modules") {
		t.Error("RunConfig candidates should include `modules`")
	}
	if !has(candidatesForType("config.Step"), "when") {
		t.Error("Step candidates should include `when`")
	}
	// Unknown container falls back to Step's vocabulary rather than
	// returning nothing.
	if len(candidatesForType("")) == 0 {
		t.Error("unknown type should still yield candidates")
	}
}

// The message the operator actually reads.
func TestFormatStrictFieldError_NamesTheFix(t *testing.T) {
	msg := formatStrictFieldError(
		"yaml: unmarshal errors:\n  line 4: field become not found in type config.Step")
	want := "unknown field `become` (did you mean `as_user`?)"
	if msg != want {
		t.Errorf("got %q, want %q", msg, want)
	}

	// No close candidate: fall back to a pointer at a surface that
	// exists. The old text named docs-next/, which was archived.
	msg = formatStrictFieldError(
		"yaml: unmarshal errors:\n  line 4: field frobnicate not found in type config.Step")
	if want := "unknown field `frobnicate` (likely a typo or a renamed field — run `mooncake actions list`)"; msg != want {
		t.Errorf("got %q, want %q", msg, want)
	}
}
