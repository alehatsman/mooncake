package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// ------ SynthesizeCompound ---------------------------------------------------

func TestSynthesizeCompound_NotAParent(t *testing.T) {
	// Leaf step: no Transaction / no Try → nil.
	leaf := config.Step{ID: "s1", Name: "leaf"}
	if got := SynthesizeCompound(leaf, []config.Step{leaf}); got != nil {
		t.Errorf("SynthesizeCompound on leaf step returned %+v, want nil", got)
	}
}

func TestSynthesizeCompound_TxnChildIsNotAParent(t *testing.T) {
	// A child step carries Transaction == nil and TxnRole != ""; the
	// parent carries Transaction != nil and TxnRole == "". This guards
	// against "child step with Transaction populated" — which
	// shouldn't happen, but the role-tag check prevents false matches.
	child := config.Step{ID: "c1", Name: "child", TxnParent: "p1", TxnRole: "body"}
	if got := SynthesizeCompound(child, []config.Step{child}); got != nil {
		t.Errorf("SynthesizeCompound on txn child returned %+v, want nil", got)
	}
}

func TestSynthesizeCompound_Transaction_TwoBodyOneRollback(t *testing.T) {
	parent := config.Step{
		ID:          "p1",
		Name:        "deploy",
		Transaction: []config.Step{{Name: "build"}, {Name: "push"}},
		OnRollback:  []config.Step{{Name: "cleanup"}},
	}
	body1 := config.Step{ID: "c1", Name: "build", TxnParent: "p1", TxnRole: "body"}
	body2 := config.Step{ID: "c2", Name: "push", TxnParent: "p1", TxnRole: "body"}
	rollback := config.Step{ID: "c3", Name: "cleanup", TxnParent: "p1", TxnRole: "rollback"}
	siblings := []config.Step{parent, body1, body2, rollback}

	got := SynthesizeCompound(parent, siblings)
	if got == nil {
		t.Fatal("SynthesizeCompound returned nil for txn parent")
	}
	if got.Resource.Attributes["kind"] != "transaction" {
		t.Errorf("kind attribute = %q, want transaction", got.Resource.Attributes["kind"])
	}
	td, ok := got.After.(*actions.TransactionDiff)
	if !ok {
		t.Fatalf("After is not *TransactionDiff; got %T", got.After)
	}
	if td.Name != "deploy" {
		t.Errorf("Name = %q, want deploy", td.Name)
	}
	if len(td.BodyChildren) != 2 || td.BodyChildren[0] != "build" || td.BodyChildren[1] != "push" {
		t.Errorf("BodyChildren = %v, want [build push]", td.BodyChildren)
	}
	if len(td.RollbackChildren) != 1 || td.RollbackChildren[0] != "cleanup" {
		t.Errorf("RollbackChildren = %v, want [cleanup]", td.RollbackChildren)
	}
}

func TestSynthesizeCompound_Try_ThreeBranches(t *testing.T) {
	parent := config.Step{
		ID:      "p1",
		Name:    "fetch-validate",
		Try:     []config.Step{{Name: "fetch"}, {Name: "validate"}},
		Catch:   []config.Step{{Name: "alert"}},
		Finally: []config.Step{{Name: "cleanup"}},
	}
	siblings := []config.Step{
		parent,
		{ID: "c1", Name: "fetch", TryParent: "p1", TryRole: "try"},
		{ID: "c2", Name: "validate", TryParent: "p1", TryRole: "try"},
		{ID: "c3", Name: "alert", TryParent: "p1", TryRole: "catch"},
		{ID: "c4", Name: "cleanup", TryParent: "p1", TryRole: "finally"},
	}

	got := SynthesizeCompound(parent, siblings)
	if got == nil {
		t.Fatal("SynthesizeCompound returned nil for try parent")
	}
	if got.Resource.Attributes["kind"] != "try" {
		t.Errorf("kind attribute = %q, want try", got.Resource.Attributes["kind"])
	}
	td, ok := got.After.(*actions.TryDiff)
	if !ok {
		t.Fatalf("After is not *TryDiff; got %T", got.After)
	}
	if len(td.TryChildren) != 2 || td.TryChildren[0] != "fetch" {
		t.Errorf("TryChildren = %v, want [fetch validate]", td.TryChildren)
	}
	if len(td.CatchChildren) != 1 || td.CatchChildren[0] != "alert" {
		t.Errorf("CatchChildren = %v, want [alert]", td.CatchChildren)
	}
	if len(td.FinallyChildren) != 1 || td.FinallyChildren[0] != "cleanup" {
		t.Errorf("FinallyChildren = %v, want [cleanup]", td.FinallyChildren)
	}
}

func TestSynthesizeCompound_OtherParentsChildrenIgnored(t *testing.T) {
	// Two transactions in the same plan; SynthesizeCompound for p1
	// must not pick up p2's children.
	p1 := config.Step{ID: "p1", Name: "t1", Transaction: []config.Step{{Name: "a"}}}
	p2 := config.Step{ID: "p2", Name: "t2", Transaction: []config.Step{{Name: "b"}}}
	siblings := []config.Step{
		p1, p2,
		{ID: "c1", Name: "a", TxnParent: "p1", TxnRole: "body"},
		{ID: "c2", Name: "b", TxnParent: "p2", TxnRole: "body"},
	}

	got := SynthesizeCompound(p1, siblings)
	td := got.After.(*actions.TransactionDiff)
	if len(td.BodyChildren) != 1 || td.BodyChildren[0] != "a" {
		t.Errorf("p1 BodyChildren = %v, want [a]", td.BodyChildren)
	}
}

// ------ matchTransaction / render --------------------------------------------

func TestLookup_Transaction_Renders(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "deploy",
			Attributes: map[string]string{"kind": "transaction"},
		},
		Operation: actions.OpUpdate,
		After: &actions.TransactionDiff{
			Name:             "deploy",
			BodyChildren:     []string{"build", "push"},
			RollbackChildren: []string{"cleanup"},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "transaction" {
		t.Errorf("Kind = %q, want transaction", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  transaction deploy (2 body, 1 rollback)\n" +
		"    body:\n" +
		"      - build\n" +
		"      - push\n" +
		"    on_rollback:\n" +
		"      - cleanup\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_Transaction_AllowIrreversibleTag(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "transaction"},
		},
		After: &actions.TransactionDiff{
			Name:              "demo",
			AllowIrreversible: true,
			BodyChildren:      []string{"x"},
		},
	}
	r := Lookup(d)
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if !strings.Contains(buf.String(), "[allow_irreversible]") {
		t.Errorf("Render missing allow_irreversible tag:\n%s", buf.String())
	}
}

func TestLookup_Transaction_OmitsEmptyRollback(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "transaction"},
		},
		After: &actions.TransactionDiff{
			Name:         "tx",
			BodyChildren: []string{"a"},
		},
	}
	r := Lookup(d)
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if strings.Contains(buf.String(), "on_rollback:") {
		t.Errorf("Render unexpectedly included on_rollback:\n%s", buf.String())
	}
}

func TestLookup_Transaction_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "transaction"},
		},
		Operation: actions.OpUpdate,
		After: map[string]interface{}{
			"name":          "deploy",
			"body_children": []interface{}{"build", "push"},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded transaction diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if !strings.Contains(buf.String(), "transaction deploy") ||
		!strings.Contains(buf.String(), "- build") ||
		!strings.Contains(buf.String(), "- push") {
		t.Errorf("Render = %q", buf.String())
	}
}

// ------ matchTry / render ----------------------------------------------------

func TestLookup_Try_Renders(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "fetch-validate",
			Attributes: map[string]string{"kind": "try"},
		},
		Operation: actions.OpUpdate,
		After: &actions.TryDiff{
			Name:            "fetch-validate",
			TryChildren:     []string{"fetch", "validate"},
			CatchChildren:   []string{"alert"},
			FinallyChildren: []string{"cleanup"},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "try" {
		t.Errorf("Kind = %q, want try", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  try fetch-validate (2 try, 1 catch, 1 finally)\n" +
		"    try:\n" +
		"      - fetch\n" +
		"      - validate\n" +
		"    catch:\n" +
		"      - alert\n" +
		"    finally:\n" +
		"      - cleanup\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_Try_OmitsEmptyBranches(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "try"},
		},
		After: &actions.TryDiff{
			Name:        "demo",
			TryChildren: []string{"x"},
		},
	}
	r := Lookup(d)
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	out := buf.String()
	if strings.Contains(out, "catch:") || strings.Contains(out, "finally:") {
		t.Errorf("Render unexpectedly included empty branches:\n%s", out)
	}
}

func TestLookup_Try_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "try"},
		},
		After: map[string]interface{}{
			"name":             "x",
			"try_children":     []interface{}{"a"},
			"catch_children":   []interface{}{"b"},
			"finally_children": []interface{}{"c"},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded try diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	out := buf.String()
	for _, want := range []string{"try x", "- a", "- b", "- c"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render missing %q in:\n%s", want, out)
		}
	}
}

// ------ helpers --------------------------------------------------------------

func TestStringSlice(t *testing.T) {
	cases := []struct {
		in   any
		want []string
	}{
		{[]interface{}{"a", "b"}, []string{"a", "b"}},
		{[]interface{}{"a", 1, "b"}, []string{"a", "b"}}, // non-string skipped
		{nil, nil},
		{"not-a-slice", nil},
	}
	for i, tc := range cases {
		got := stringSlice(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("case %d: len = %d, want %d", i, len(got), len(tc.want))
			continue
		}
		for j := range got {
			if got[j] != tc.want[j] {
				t.Errorf("case %d: [%d] = %q, want %q", i, j, got[j], tc.want[j])
			}
		}
	}
}
