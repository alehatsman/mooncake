package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// ------ helpers --------------------------------------------------------------

func gitCheckoutDiff(op actions.Operation, before, after *actions.GitCheckoutDiff) *actions.Diff {
	dest := ""
	if after != nil {
		dest = after.Dest
	}
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceGit,
			Identifier: dest,
			Attributes: map[string]string{"kind": "git.checkout"},
		},
		Operation: op,
		After:     after,
	}
	if before != nil {
		d.Before = before
	}
	return d
}

func gitConfigDiff(op actions.Operation, payload *actions.GitConfigDiff) *actions.Diff {
	id := payload.Scope
	if payload.Scope == "local" && payload.Repo != "" {
		id = payload.Scope + ":" + payload.Repo
	}
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceGit,
			Identifier: id,
			Attributes: map[string]string{"kind": "git.config"},
		},
		Operation: op,
		After:     payload,
	}
}

func repoDiff(op actions.Operation, payload *actions.RepoDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: payload.Name,
			Attributes: map[string]string{"kind": "pkg.repo"},
		},
		Operation: op,
		After:     payload,
	}
}

// ------ git.checkout ---------------------------------------------------------

func TestLookup_GitCheckout_UpdateWithBefore(t *testing.T) {
	r := Lookup(gitCheckoutDiff(actions.OpUpdate,
		&actions.GitCheckoutDiff{Dest: "/srv/app", HeadSHA: "abc1234567890"},
		&actions.GitCheckoutDiff{Dest: "/srv/app", Ref: "v1.2.3"},
	))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "git.checkout" {
		t.Errorf("Kind = %q, want git.checkout", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  git.checkout update /srv/app to v1.2.3 (from abc1234)\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_GitCheckout_UpdateWithoutBefore(t *testing.T) {
	// Missing dest case — handler sets Before=nil.
	r := Lookup(gitCheckoutDiff(actions.OpUpdate, nil,
		&actions.GitCheckoutDiff{Dest: "/srv/new", Ref: "main"},
	))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  git.checkout update /srv/new to main\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GitCheckout_Noop(t *testing.T) {
	r := Lookup(gitCheckoutDiff(actions.OpNoop,
		&actions.GitCheckoutDiff{Dest: "/srv/app", HeadSHA: "abc1234"},
		&actions.GitCheckoutDiff{Dest: "/srv/app", Ref: "v1.2.3"},
	))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	// noop shows "at" instead of "to" and suppresses the from-SHA.
	if got := buf.String(); got != "  git.checkout noop /srv/app at v1.2.3\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GitCheckout_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceGit,
			Attributes: map[string]string{"kind": "git.checkout"},
		},
		Operation: actions.OpUpdate,
		After: map[string]interface{}{
			"dest": "/srv/app",
			"ref":  "v2",
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded git.checkout diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); !strings.Contains(got, "/srv/app to v2") {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GitCheckout_WrongAttribute(t *testing.T) {
	d := gitCheckoutDiff(actions.OpUpdate, nil,
		&actions.GitCheckoutDiff{Dest: "/x", Ref: "y"})
	d.Resource.Attributes["kind"] = "git.config"
	if r := Lookup(d); r != nil && r.Kind() == "git.checkout" {
		t.Errorf("git.checkout renderer matched on kind=git.config: %+v", r)
	}
}

// ------ git.config -----------------------------------------------------------

func TestLookup_GitConfig_GlobalUpdate(t *testing.T) {
	r := Lookup(gitConfigDiff(actions.OpUpdate, &actions.GitConfigDiff{
		Scope: "global",
		Entries: []actions.GitConfigEntry{
			{Key: "user.email", Value: "dev@example.com", Op: "set"},
			{Key: "push.default", Op: "unset"},
		},
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "git.config" {
		t.Errorf("Kind = %q, want git.config", r.Kind())
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	want := "  git.config update global\n" +
		"    set user.email = dev@example.com\n" +
		"    unset push.default\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_GitConfig_LocalShowsRepo(t *testing.T) {
	r := Lookup(gitConfigDiff(actions.OpUpdate, &actions.GitConfigDiff{
		Scope: "local",
		Repo:  "/srv/myrepo",
		Entries: []actions.GitConfigEntry{
			{Key: "core.hooksPath", Value: ".githooks", Op: "set"},
		},
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	want := "  git.config update local /srv/myrepo\n" +
		"    set core.hooksPath = .githooks\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_GitConfig_Noop_EmptyEntries(t *testing.T) {
	r := Lookup(gitConfigDiff(actions.OpNoop, &actions.GitConfigDiff{Scope: "global"}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  git.config noop global\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GitConfig_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceGit,
			Attributes: map[string]string{"kind": "git.config"},
		},
		Operation: actions.OpUpdate,
		After: map[string]interface{}{
			"scope": "global",
			"entries": []interface{}{
				map[string]interface{}{"key": "user.name", "value": "Alice", "op": "set"},
			},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded git.config diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); !strings.Contains(got, "set user.name = Alice") {
		t.Errorf("Render = %q", got)
	}
}

// ------ pkg.repo -------------------------------------------------------------

func TestLookup_Repo_AddWithDriver(t *testing.T) {
	r := Lookup(repoDiff(actions.OpCreate, &actions.RepoDiff{
		Name:   "docker",
		State:  "present",
		Driver: "apt",
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "pkg.repo" {
		t.Errorf("Kind = %q, want pkg.repo", r.Kind())
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  pkg.repo add docker (apt)\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_Repo_RemoveNoDriver(t *testing.T) {
	r := Lookup(repoDiff(actions.OpDelete, &actions.RepoDiff{
		Name:  "old-repo",
		State: "absent",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  pkg.repo remove old-repo\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_Repo_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Attributes: map[string]string{"kind": "pkg.repo"},
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"name":   "nginx-stable",
			"state":  "present",
			"driver": "dnf",
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded repo diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  pkg.repo add nginx-stable (dnf)\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_Repo_WrongAttribute(t *testing.T) {
	d := repoDiff(actions.OpCreate, &actions.RepoDiff{Name: "x"})
	d.Resource.Attributes["kind"] = "pkg.upgrade"
	if r := Lookup(d); r != nil && r.Kind() == "pkg.repo" {
		t.Errorf("pkg.repo renderer matched on kind=pkg.upgrade: %+v", r)
	}
}

func TestLookup_Repo_EmptyAfter(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Attributes: map[string]string{"kind": "pkg.repo"},
		},
		After: map[string]interface{}{},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on empty After: %+v", r)
	}
}

func TestShortSHA(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc1234567890def", "abc1234"},
		{"abc12", "abc12"},
		{"  abc1234567890  ", "abc1234"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := shortSHA(tc.in); got != tc.want {
			t.Errorf("shortSHA(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
