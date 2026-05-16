package diff

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/effects"
)

func TestLookup_NilDetail(t *testing.T) {
	if r := Lookup(nil); r != nil {
		t.Errorf("Lookup(nil) = %v, want nil", r)
	}
}

func TestLookup_UnknownDetail(t *testing.T) {
	type weirdShape struct{ X int }
	if r := Lookup(weirdShape{X: 1}); r != nil {
		t.Errorf("Lookup(unknown shape) = %v, want nil", r)
	}
}

func TestLookup_ContentDiff_InMemory(t *testing.T) {
	cd := effects.ContentDiff{
		UnifiedDiff: "--- a/x\n+++ b/x\n@@ -1,1 +1,1 @@\n-old\n+new\n",
	}
	r := Lookup(cd)
	if r == nil {
		t.Fatal("Lookup(effects.ContentDiff) returned nil")
	}
	if r.Kind() != "file" {
		t.Errorf("Kind() = %q, want %q", r.Kind(), "file")
	}
}

func TestLookup_ContentDiff_FromJSON(t *testing.T) {
	// Round-tripped JSON plan shape: Detail decoded as map[string]interface{}.
	detail := map[string]interface{}{
		"unified_diff": "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n",
		"old_size":     float64(3),
		"new_size":     float64(3),
	}
	r := Lookup(detail)
	if r == nil {
		t.Fatal("Lookup(json-shaped map) returned nil")
	}
	if r.Kind() != "file" {
		t.Errorf("Kind() = %q, want %q", r.Kind(), "file")
	}
}

func TestLookup_EmptyUnifiedDiff_ReturnsNil(t *testing.T) {
	// Binary files / new files set OldHash/NewHash but leave UnifiedDiff
	// empty. Plan output should show no diff body for these — the
	// renderer treats empty diff as "no Renderer", letting the caller's
	// placeholder text take over.
	if r := Lookup(effects.ContentDiff{OldSize: 0, NewSize: 100}); r != nil {
		t.Errorf("Lookup(empty unified) = %v, want nil for empty diff body", r)
	}
	if r := Lookup(map[string]interface{}{"unified_diff": ""}); r != nil {
		t.Errorf("Lookup(empty unified map) = %v, want nil", r)
	}
}

// Render text path: lines are indented by two spaces, exactly as
// cmd/mooncake.go formatPlanText did pre-this-PR. Lock this output
// byte-for-byte — it's the wave-1 "zero behavior change" contract.
func TestFileRenderer_Render_Text(t *testing.T) {
	cd := effects.ContentDiff{
		UnifiedDiff: "--- a/x\n+++ b/x\n@@ -1,1 +1,1 @@\n-old\n+new",
	}
	r := Lookup(cd)
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  --- a/x\n  +++ b/x\n  @@ -1,1 +1,1 @@\n  -old\n  +new\n"
	if got := buf.String(); got != want {
		t.Errorf("Render text mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// A unified diff with a trailing newline must still emit each non-empty
// line exactly once — no double-newline at the end. The pre-PR cmd
// path used strings.TrimRight(udiff, "\n") then split; we preserve
// that behavior.
func TestFileRenderer_Render_TrailingNewlineNormalised(t *testing.T) {
	cd := effects.ContentDiff{UnifiedDiff: "line1\nline2\n"}
	r := Lookup(cd)
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  line1\n  line2\n"
	if got := buf.String(); got != want {
		t.Errorf("Render normalised newline mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestFileRenderer_Render_EmptyFormatIsText(t *testing.T) {
	// "" (zero value of Format) is treated as FormatText so callers
	// can pass an unset format and get sensible default behavior.
	cd := effects.ContentDiff{UnifiedDiff: "single"}
	r := Lookup(cd)
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, ""); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  single\n" {
		t.Errorf("Render default format mismatch: %q", got)
	}
}

func TestFileRenderer_Render_JSONFallback(t *testing.T) {
	// JSON/YAML callers currently serialise the raw Plan struct;
	// invoking Render in those formats falls back to the raw unified
	// diff text. Locked here so a future caller invoking
	// Render(FormatJSON) gets something usable rather than an error.
	cd := effects.ContentDiff{UnifiedDiff: "x\n"}
	r := Lookup(cd)
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatJSON); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.String() != "x\n" {
		t.Errorf("FormatJSON fallback = %q, want %q", buf.String(), "x\n")
	}
}

func TestFileRenderer_Render_UnsupportedFormat(t *testing.T) {
	cd := effects.ContentDiff{UnifiedDiff: "x"}
	r := Lookup(cd)
	err := r.Render(io.Discard, Format("toml"))
	if err == nil {
		t.Error("expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "toml") {
		t.Errorf("expected error to mention the format, got %q", err.Error())
	}
}

// TestRegistry_Order — registered matchFuncs are tried in registration
// order, first match wins. Today only the file matcher is registered;
// asserting the contract here defends future renderer additions from
// silently reordering precedence.
func TestRegistry_Order(t *testing.T) {
	// snapshot+restore the registry around the test
	prev := registry
	defer func() { registry = prev }()

	type tag struct{ name string }
	first := &fileRenderer{unified: "first-match"}
	second := &fileRenderer{unified: "second-match"}

	registry = nil
	Register(func(d any) Renderer {
		if _, ok := d.(tag); ok {
			return first
		}
		return nil
	})
	Register(func(d any) Renderer {
		if _, ok := d.(tag); ok {
			return second
		}
		return nil
	})

	r := Lookup(tag{name: "x"})
	fr, ok := r.(*fileRenderer)
	if !ok || fr.unified != "first-match" {
		t.Errorf("registry first-match contract broken: got %+v", r)
	}
}
