package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// ------ helpers --------------------------------------------------------------

func userDiff(op actions.Operation, payload *actions.UserDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: payload.Name,
			Attributes: map[string]string{"kind": "os.user"},
		},
		Operation: op,
		After:     payload,
	}
}

func groupDiff(op actions.Operation, payload *actions.GroupDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: payload.Name,
			Attributes: map[string]string{"kind": "os.group"},
		},
		Operation: op,
		After:     payload,
	}
}

func intp(i int) *int { return &i }

// ------ user -----------------------------------------------------------------

func TestLookup_UserDiff_CreateWithAttrs(t *testing.T) {
	r := Lookup(userDiff(actions.OpCreate, &actions.UserDiff{
		Name:   "alice",
		State:  "present",
		UID:    intp(1001),
		Shell:  "/bin/zsh",
		Home:   "/home/alice",
		Groups: []string{"docker", "sudo"},
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "user" {
		t.Errorf("Kind = %q, want user", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  user create alice (uid=1001 shell=/bin/zsh home=/home/alice groups=docker,sudo)\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_UserDiff_RemoveBare(t *testing.T) {
	r := Lookup(userDiff(actions.OpDelete, &actions.UserDiff{Name: "alice", State: "absent"}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  user remove alice\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_UserDiff_SystemAccount(t *testing.T) {
	r := Lookup(userDiff(actions.OpCreate, &actions.UserDiff{
		Name:   "deploy",
		State:  "present",
		System: true,
		Groups: []string{"deploy"},
	}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  user create deploy (system groups=deploy)\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_UserDiff_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "alice",
			Attributes: map[string]string{"kind": "os.user"},
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"name":   "alice",
			"state":  "present",
			"uid":    float64(1001),
			"shell":  "/bin/zsh",
			"groups": []interface{}{"docker"},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded user diff")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "user create alice") || !strings.Contains(got, "uid=1001") {
		t.Errorf("json-decoded render missing expected attrs: %q", got)
	}
}

func TestLookup_UserDiff_WrongAttribute(t *testing.T) {
	// Same shape but kind attribute says "os.group" → user renderer
	// must pass. (Group renderer will pick it up; that's a separate
	// test.)
	d := userDiff(actions.OpCreate, &actions.UserDiff{Name: "x"})
	d.Resource.Attributes["kind"] = "os.group"
	if r := Lookup(d); r != nil && r.Kind() == "user" {
		t.Errorf("user renderer matched on kind=os.group: %+v", r)
	}
}

func TestLookup_UserDiff_EmptyAfter(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.user"},
		},
		After: map[string]interface{}{},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on empty after: %+v", r)
	}
}

// ------ group ----------------------------------------------------------------

func TestLookup_GroupDiff_Create(t *testing.T) {
	r := Lookup(groupDiff(actions.OpCreate, &actions.GroupDiff{
		Name:  "deploy",
		State: "present",
		GID:   intp(1001),
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "group" {
		t.Errorf("Kind = %q, want group", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  group create deploy (gid=1001)\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GroupDiff_Remove(t *testing.T) {
	r := Lookup(groupDiff(actions.OpDelete, &actions.GroupDiff{Name: "staff", State: "absent"}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  group remove staff\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GroupDiff_SystemFlag(t *testing.T) {
	r := Lookup(groupDiff(actions.OpCreate, &actions.GroupDiff{
		Name:   "docker",
		State:  "present",
		System: true,
	}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  group create docker (system)\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GroupDiff_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "staff",
			Attributes: map[string]string{"kind": "os.group"},
		},
		Operation: actions.OpDelete,
		After: map[string]interface{}{
			"name":  "staff",
			"state": "absent",
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded group diff")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  group remove staff\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GroupDiff_WrongAttribute(t *testing.T) {
	d := groupDiff(actions.OpCreate, &actions.GroupDiff{Name: "x"})
	d.Resource.Attributes["kind"] = "os.user"
	r := Lookup(d)
	if r != nil && r.Kind() == "group" {
		t.Errorf("group renderer matched on kind=os.user: %+v", r)
	}
}

func TestLookup_GroupDiff_EmptyAfter(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.group"},
		},
		After: map[string]interface{}{},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on empty after: %+v", r)
	}
}

// userOperationVerb is shared between renderers — lock its mapping
// here (also covers group rendering since groupRenderer uses the
// same function).
func TestUserOperationVerb(t *testing.T) {
	cases := []struct {
		op   actions.Operation
		want string
	}{
		{actions.OpCreate, "create"},
		{actions.OpUpdate, "update"},
		{actions.OpDelete, "remove"},
		{actions.OpNoop, "noop"},
		{actions.Operation("weird"), "change"},
	}
	for _, tc := range cases {
		if got := userOperationVerb(tc.op); got != tc.want {
			t.Errorf("userOperationVerb(%q) = %q, want %q", tc.op, got, tc.want)
		}
	}
}
