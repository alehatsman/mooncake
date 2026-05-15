package pathquery

import (
	"strings"
	"testing"
)

func TestExtract_EmptyPathReturnsWholeDoc(t *testing.T) {
	doc := map[string]any{"a": 1}
	got, ok, err := Extract(doc, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("found should be true for empty path")
	}
	m, isMap := got.(map[string]any)
	if !isMap || m["a"] != 1 {
		t.Errorf("expected whole doc back, got %v", got)
	}
}

func TestExtract_ScalarAtRoot(t *testing.T) {
	doc := map[string]any{"version": "1.2.3"}
	got, ok, err := Extract(doc, "version")
	if err != nil || !ok || got != "1.2.3" {
		t.Errorf("version: got=%v ok=%v err=%v", got, ok, err)
	}
}

func TestExtract_NestedDotted(t *testing.T) {
	doc := map[string]any{
		"service": map[string]any{"port": 8080},
	}
	got, ok, err := Extract(doc, "service.port")
	if err != nil || !ok || got != 8080 {
		t.Errorf("service.port: got=%v ok=%v err=%v", got, ok, err)
	}
}

func TestExtract_ArrayIndex(t *testing.T) {
	doc := map[string]any{
		"tools": []any{
			map[string]any{"name": "go"},
			map[string]any{"name": "gopls"},
		},
	}
	got, ok, err := Extract(doc, "tools[1].name")
	if err != nil || !ok || got != "gopls" {
		t.Errorf("tools[1].name: got=%v ok=%v err=%v", got, ok, err)
	}
}

func TestExtract_ChainedIndices(t *testing.T) {
	doc := map[string]any{
		"matrix": []any{
			[]any{1, 2, 3},
			[]any{4, 5, 6},
		},
	}
	got, ok, err := Extract(doc, "matrix[1][2]")
	if err != nil || !ok || got != 6 {
		t.Errorf("matrix[1][2]: got=%v ok=%v err=%v", got, ok, err)
	}
}

func TestExtract_KeyMissReturnsNotFoundNoErr(t *testing.T) {
	doc := map[string]any{"a": 1}
	got, ok, err := Extract(doc, "b")
	if err != nil {
		t.Fatalf("missing key should not error, got %v", err)
	}
	if ok {
		t.Errorf("found should be false on miss, got=%v", got)
	}
	if got != nil {
		t.Errorf("missed lookup should return nil value, got %v", got)
	}
}

func TestExtract_IndexOutOfRangeReturnsNotFoundNoErr(t *testing.T) {
	doc := map[string]any{"xs": []any{1, 2, 3}}
	got, ok, err := Extract(doc, "xs[7]")
	if err != nil {
		t.Fatalf("oob index should not error, got %v", err)
	}
	if ok || got != nil {
		t.Errorf("oob should return (nil,false,nil), got (%v,%v)", got, ok)
	}
}

func TestExtract_TypeMismatchErrors(t *testing.T) {
	doc := map[string]any{"a": 1}
	_, _, err := Extract(doc, "a.b")
	if err == nil {
		t.Fatal("expected error for dotting into a scalar")
	}
	if !strings.Contains(err.Error(), "non-object") {
		t.Errorf("unhelpful error: %v", err)
	}

	doc2 := map[string]any{"a": map[string]any{"b": 1}}
	_, _, err = Extract(doc2, "a[0]")
	if err == nil {
		t.Fatal("expected error for indexing a map")
	}
	if !strings.Contains(err.Error(), "non-array") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestValidate_AcceptsSupportedSyntax(t *testing.T) {
	good := []string{
		"",
		"a",
		"a.b",
		"a.b.c",
		"a[0]",
		"a[0][1]",
		"a.b[3].c",
		"a_b-c",
	}
	for _, p := range good {
		if err := Validate(p); err != nil {
			t.Errorf("Validate(%q) unexpected error: %v", p, err)
		}
	}
}

func TestValidate_RejectsUnsupportedSyntax(t *testing.T) {
	cases := []struct {
		path    string
		wantSub string
	}{
		{".a", "leading dot"},
		{"a.", "trailing dot"},
		{"a..b", "empty segment"},
		{"a[?@.b == 1]", "unsupported filter"},
		{"a[*]", "unsupported filter"},
		{"a[-1]", "negative index"},
		{"a[]", "empty index"},
		{"a[", "unmatched"},
		{"$.a", "unsupported character"},
		{"a*", "unsupported character"},
		{"3a", "starts with a digit"},
	}
	for _, c := range cases {
		err := Validate(c.path)
		if err == nil {
			t.Errorf("Validate(%q) should have errored", c.path)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("Validate(%q) error %q lacks %q", c.path, err.Error(), c.wantSub)
		}
	}
}

func TestExtract_PropagatesMalformedSyntaxError(t *testing.T) {
	_, _, err := Extract(map[string]any{}, "a..b")
	if err == nil {
		t.Fatal("malformed path should error from Extract too")
	}
}
