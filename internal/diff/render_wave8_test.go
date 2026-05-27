package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// ------ git.clone -----------------------------------------------------------

func gitCloneDiff(op actions.Operation, before, after *actions.GitCloneDiff) *actions.Diff {
	dest := ""
	if after != nil {
		dest = after.Dest
	}
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceGit,
			Identifier: dest,
			Attributes: map[string]string{"kind": "git.clone"},
		},
		Operation: op,
		After:     after,
	}
	if before != nil {
		d.Before = before
	}
	return d
}

func TestLookup_GitClone_Create(t *testing.T) {
	r := Lookup(gitCloneDiff(actions.OpCreate, nil,
		&actions.GitCloneDiff{Dest: "/srv/app", Repo: "https://github.com/x/y", Ref: "main"},
	))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "git.clone" {
		t.Errorf("Kind = %q, want git.clone", r.Kind())
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	want := "  git.clone create /srv/app from https://github.com/x/y at main\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_GitClone_UpdateWithBefore(t *testing.T) {
	r := Lookup(gitCloneDiff(actions.OpUpdate,
		&actions.GitCloneDiff{Dest: "/srv/app", HeadSHA: "abc1234567"},
		&actions.GitCloneDiff{Dest: "/srv/app", Repo: "https://github.com/x/y", Ref: "v1.2.3"},
	))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	want := "  git.clone update /srv/app from https://github.com/x/y at v1.2.3 (from abc1234)\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_GitClone_Noop(t *testing.T) {
	r := Lookup(gitCloneDiff(actions.OpNoop,
		&actions.GitCloneDiff{Dest: "/srv/app", HeadSHA: "abc1234"},
		&actions.GitCloneDiff{Dest: "/srv/app", Ref: "main"},
	))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  git.clone noop /srv/app at main\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_GitClone_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceGit,
			Attributes: map[string]string{"kind": "git.clone"},
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"dest": "/srv/app",
			"repo": "https://github.com/x/y",
			"ref":  "main",
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded git.clone diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if !strings.Contains(buf.String(), "create /srv/app") {
		t.Errorf("Render = %q", buf.String())
	}
}

// ------ os.ssh_key ----------------------------------------------------------

func sshKeyDiff(op actions.Operation, payload *actions.SSHKeyDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: payload.User,
			Attributes: map[string]string{"kind": "os.ssh_key"},
		},
		Operation: op,
		After:     payload,
	}
}

func TestLookup_SshKey_AddMultipleKeys(t *testing.T) {
	r := Lookup(sshKeyDiff(actions.OpCreate, &actions.SSHKeyDiff{
		User:     "deploy",
		State:    "present",
		KeyCount: 3,
	}))
	if r == nil || r.Kind() != "os.ssh_key" {
		t.Fatalf("Kind mismatch or nil; got %v", r)
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  os.ssh_key add deploy: 3 keys\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_SshKey_AddSingleKeyWithExclusiveAndPath(t *testing.T) {
	r := Lookup(sshKeyDiff(actions.OpCreate, &actions.SSHKeyDiff{
		User:      "deploy",
		KeyCount:  1,
		Path:      "/etc/ssh/keys",
		Exclusive: true,
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	want := "  os.ssh_key add deploy: 1 key at /etc/ssh/keys [exclusive]\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_SshKey_Remove_IdentityOnly(t *testing.T) {
	r := Lookup(sshKeyDiff(actions.OpDelete, &actions.SSHKeyDiff{
		User:     "deploy",
		State:    "absent",
		KeyCount: 2,
		Path:     "/etc/ssh/keys",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  os.ssh_key remove deploy\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_SshKey_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.ssh_key"},
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"user":      "deploy",
			"key_count": float64(2),
			"exclusive": true,
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded ssh_key diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if !strings.Contains(buf.String(), "deploy: 2 keys") ||
		!strings.Contains(buf.String(), "[exclusive]") {
		t.Errorf("Render = %q", buf.String())
	}
}

// ------ os.sysctl -----------------------------------------------------------

func sysctlDiff(op actions.Operation, payload *actions.SysctlDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: payload.Name,
			Attributes: map[string]string{"kind": "os.sysctl"},
		},
		Operation: op,
		After:     payload,
	}
}

func TestLookup_Sysctl_Set(t *testing.T) {
	r := Lookup(sysctlDiff(actions.OpUpdate, &actions.SysctlDiff{
		Name:  "net.ipv4.ip_forward",
		State: "present",
		Value: "1",
	}))
	if r == nil || r.Kind() != "os.sysctl" {
		t.Fatalf("Kind mismatch or nil; got %v", r)
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  os.sysctl set net.ipv4.ip_forward = 1\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_Sysctl_Remove(t *testing.T) {
	r := Lookup(sysctlDiff(actions.OpDelete, &actions.SysctlDiff{
		Name:  "net.ipv4.ip_forward",
		State: "absent",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  os.sysctl remove net.ipv4.ip_forward\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_Sysctl_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.sysctl"},
		},
		Operation: actions.OpUpdate,
		After: map[string]interface{}{
			"name":  "vm.swappiness",
			"state": "present",
			"value": "10",
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded sysctl diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  os.sysctl set vm.swappiness = 10\n" {
		t.Errorf("Render = %q", got)
	}
}

// ------ pkg.upgrade ---------------------------------------------------------

func pkgUpgradeDiff(payload *actions.PkgUpgradeDiff) *actions.Diff {
	identifier := "<system>"
	if !payload.FullUpgrade && len(payload.Names) > 0 {
		identifier = payload.Names[0]
	}
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: identifier,
			Attributes: map[string]string{"kind": "pkg.upgrade"},
		},
		Operation: actions.OpUpdate,
		After:     payload,
	}
}

func TestLookup_PkgUpgrade_FullSystem(t *testing.T) {
	r := Lookup(pkgUpgradeDiff(&actions.PkgUpgradeDiff{
		Manager:     "apt",
		FullUpgrade: true,
		Autoremove:  true,
	}))
	if r == nil || r.Kind() != "pkg.upgrade" {
		t.Fatalf("Kind mismatch or nil; got %v", r)
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  pkg.upgrade apt: <system> [autoremove]\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_PkgUpgrade_SubsetNames(t *testing.T) {
	r := Lookup(pkgUpgradeDiff(&actions.PkgUpgradeDiff{
		Manager: "dnf",
		Names:   []string{"nginx", "redis"},
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  pkg.upgrade dnf: nginx, redis\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_PkgUpgrade_NoManager(t *testing.T) {
	r := Lookup(pkgUpgradeDiff(&actions.PkgUpgradeDiff{FullUpgrade: true}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  pkg.upgrade: <system>\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_PkgUpgrade_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Attributes: map[string]string{"kind": "pkg.upgrade"},
		},
		Operation: actions.OpUpdate,
		After: map[string]interface{}{
			"manager":      "apt",
			"full_upgrade": true,
			"names":        []interface{}{},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded pkg.upgrade diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if !strings.Contains(buf.String(), "apt: <system>") {
		t.Errorf("Render = %q", buf.String())
	}
}

func TestLookup_PkgUpgrade_DoesNotMatchPlainPkg(t *testing.T) {
	// matchPackage handles plain pkg (no kind attribute). The
	// pkg.upgrade matcher must NOT swallow those — guarded by
	// kind != "pkg.upgrade" gate.
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind: actions.ResourcePackage,
		},
		Operation: actions.OpCreate,
		After:     &actions.PackageDiff{Names: []string{"vim"}, State: "present"},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for plain pkg")
	}
	if r.Kind() != "package" {
		t.Errorf("plain pkg dispatched to %q, want package", r.Kind())
	}
}
