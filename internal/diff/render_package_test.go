package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// helper — build a typed actions.Diff for the pkg case.
func pkgDiff(op actions.Operation, names []string, state, manager string) *actions.Diff {
	identifier := "<unnamed>"
	if len(names) > 0 {
		identifier = names[0]
	}
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: identifier,
		},
		Operation: op,
		After: &actions.PackageDiff{
			Names:   names,
			State:   state,
			Manager: manager,
		},
	}
}

func TestLookup_PackageDiff_Install(t *testing.T) {
	d := pkgDiff(actions.OpCreate, []string{"vim"}, "present", "apt")
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for pkg diff")
	}
	if r.Kind() != "package" {
		t.Errorf("Kind() = %q, want %q", r.Kind(), "package")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  pkg install apt: vim\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_PackageDiff_Upgrade_LatestState(t *testing.T) {
	d := pkgDiff(actions.OpUpdate, []string{"terraform"}, "latest", "brew")
	r := Lookup(d)
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  pkg upgrade brew: terraform\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_PackageDiff_Remove(t *testing.T) {
	d := pkgDiff(actions.OpDelete, []string{"nginx"}, "absent", "apt")
	r := Lookup(d)
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  pkg remove apt: nginx\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_PackageDiff_MultipleNames(t *testing.T) {
	d := pkgDiff(actions.OpCreate, []string{"vim", "tmux", "git"}, "present", "")
	r := Lookup(d)
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	// No manager when not pinned; comma-separated names.
	if got := buf.String(); got != "  pkg install: vim, tmux, git\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_PackageDiff_UnnamedFallback(t *testing.T) {
	d := pkgDiff(actions.OpCreate, nil, "present", "apt")
	// Empty Names but State or Manager populated → match succeeds and
	// we render "<unnamed>" so the operator sees something rather than
	// nothing. (Handler-level validation rejects empty-name steps; this
	// is defense in depth.)
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for state+manager-only payload")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "<unnamed>") {
		t.Errorf("expected <unnamed> in output, got %q", buf.String())
	}
}

// JSON-decoded plan path: After arrives as a map[string]interface{}
// after re-loading a saved plan from disk. The renderer must still
// recognise it.
func TestLookup_PackageDiff_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: "vim",
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"names":   []interface{}{"vim"},
			"state":   "present",
			"manager": "apt",
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded pkg diff")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  pkg install apt: vim\n" {
		t.Errorf("Render = %q", got)
	}
}

// Wrong Resource.Kind on the Diff → no match. Defends against future
// non-pkg handlers accidentally winning the pkg renderer's dispatch.
func TestLookup_PackageDiff_WrongKind(t *testing.T) {
	d := &actions.Diff{
		Resource:  actions.ResourceRef{Kind: actions.ResourceFile, Identifier: "/tmp/x"},
		Operation: actions.OpCreate,
		After:     &actions.PackageDiff{Names: []string{"vim"}, State: "present"},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on non-package kind: %+v", r)
	}
}

// Empty After payload (no names, no state, no manager) → no match.
// Without any payload data the renderer would produce a meaningless
// "pkg install: <unnamed>" line. Falling back to nil lets the caller's
// generic "would update" placeholder render instead.
func TestLookup_PackageDiff_EmptyAfter(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{Kind: actions.ResourcePackage},
		After:    map[string]interface{}{},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on empty After: %+v", r)
	}
}

// Variadic Lookup contract: nil candidates are skipped and the first
// candidate any matcher recognises wins. The cmd plan-render call site
// relies on Lookup(ins.Detail, ins.Diff) — passing both because file
// steps populate Detail while pkg / os.* steps populate Diff.
func TestLookup_Variadic_FirstMatchWins(t *testing.T) {
	// First candidate: a Diff with wrong Resource.Kind (no match).
	// Second candidate: a real pkg Diff (matches).
	miss := &actions.Diff{
		Resource: actions.ResourceRef{Kind: actions.ResourceFile, Identifier: "/x"},
	}
	hit := pkgDiff(actions.OpCreate, []string{"vim"}, "present", "apt")

	r := Lookup(nil, miss, hit)
	if r == nil {
		t.Fatal("variadic Lookup returned nil; expected pkg match on third candidate")
	}
	if r.Kind() != "package" {
		t.Errorf("Kind = %q, want package", r.Kind())
	}
}

// operationVerb classification — locked here because the verbs are
// part of the user-facing wire format (plan --diff text output).
func TestOperationVerb(t *testing.T) {
	cases := []struct {
		op   actions.Operation
		want string
	}{
		{actions.OpCreate, "install"},
		{actions.OpUpdate, "upgrade"},
		{actions.OpDelete, "remove"},
		{actions.OpNoop, "noop"},
		{actions.Operation("weird"), "change"},
	}
	for _, tc := range cases {
		if got := operationVerb(tc.op); got != tc.want {
			t.Errorf("operationVerb(%q) = %q, want %q", tc.op, got, tc.want)
		}
	}
}
