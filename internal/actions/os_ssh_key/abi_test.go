package os_ssh_key //nolint:revive // package name follows action convention

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "ssh-ed25519 AAAA..."}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
}

func TestPermissions_CustomPathSurfacesInFilesystemWrite(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsSSHKey: &config.OsSSHKey{
		User: "deploy",
		Key:  "ssh-ed25519 AAAA...",
		Path: "/etc/ssh/authorized_keys.d/deploy",
	}})
	if len(ps.FilesystemWrite) != 1 || ps.FilesystemWrite[0] != "/etc/ssh/authorized_keys.d/deploy" {
		t.Errorf("FilesystemWrite = %v, want [/etc/ssh/authorized_keys.d/deploy]", ps.FilesystemWrite)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: "deploy",
		Key:  "ssh-ed25519 AAAA...",
	}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if !strings.HasPrefix(d.Resource.Identifier, "deploy:") {
		t.Errorf("Identifier = %s, want prefix 'deploy:'", d.Resource.Identifier)
	}
	if d.Resource.Attributes["user"] != "deploy" {
		t.Errorf("Attributes[user] = %s, want deploy", d.Resource.Attributes["user"])
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "ssh-ed25519 X", State: "absent"}})
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpDelete)
	}
}

func TestDiff_KeyCountIncludesSingleAndMulti(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: "deploy",
		Key:  "ssh-ed25519 ONE",
		Keys: []string{"ssh-ed25519 TWO", "ssh-ed25519 THREE"},
	}})
	after, ok := d.After.(*actions.SSHKeyDiff)
	if !ok {
		t.Fatalf("After is not *actions.SSHKeyDiff; got %T", d.After)
	}
	if after.KeyCount != 3 {
		t.Errorf("KeyCount = %d, want 3", after.KeyCount)
	}
}

func TestDiff_DoesNotLeakKeyMaterial(t *testing.T) {
	h := Handler{}
	secret := "ssh-ed25519 SUPERSECRET_PUB_KEY"
	d, _ := h.Diff(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: secret}})
	after := d.After.(*actions.SSHKeyDiff)
	// The snapshot must NOT include the key material itself — it
	// only carries count + metadata. (Public keys aren't strictly
	// secret, but Diff is a structural surface that may be logged
	// or shipped to agents; keeping key material off it is the
	// conservative choice.)
	body := after.User + after.State + after.Path
	if strings.Contains(body, secret) {
		t.Errorf("SSHKeyDiff must not contain key material; found: %s", body)
	}
	if d.Resource.Identifier == secret || strings.Contains(d.Resource.Identifier, "SUPERSECRET") {
		t.Errorf("Identifier must not contain key material; got: %s", d.Resource.Identifier)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_BaseRiskIs5(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "x"}})
	if c.Risk != 5 {
		t.Errorf("Risk = %d, want 5", c.Risk)
	}
}

func TestCost_ExclusiveBumpsRisk(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "x", Exclusive: true}})
	if c.Risk != 6 {
		t.Errorf("Risk = %d, want 6 (exclusive replaces existing keys)", c.Risk)
	}
}

func TestCost_ResourcesCountsKeys(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: "deploy",
		Keys: []string{"a", "b", "c"},
	}})
	if c.Resources != 3 {
		t.Errorf("Resources = %d, want 3", c.Resources)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_CreatedFileBecomesAbsent(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSSHKeyReverseInfo{
		Path:         "/home/deploy/.ssh/authorized_keys",
		User:         "deploy",
		PriorExisted: false,
	}
	rev, err := h.Reverse(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "x"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.FileWrite == nil {
		t.Fatal("Reverse must return a file.write step")
	}
	if rev.FileWrite.State != "absent" {
		t.Errorf("State = %s, want absent", rev.FileWrite.State)
	}
}

func TestReverse_ExistingFileContentRestored(t *testing.T) {
	h := Handler{}
	prior := "ssh-ed25519 AAAAOLDKEY existing@host\n"
	r := executor.NewResult()
	r.ReverseData = &OsSSHKeyReverseInfo{
		Path:         "/home/deploy/.ssh/authorized_keys",
		User:         "deploy",
		PriorExisted: true,
		PriorContent: prior,
	}
	rev, _ := h.Reverse(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "new"}}, r)
	if rev == nil || rev.FileWrite == nil {
		t.Fatal("Reverse must return a file.write step")
	}
	if rev.FileWrite.State != "file" {
		t.Errorf("State = %s, want file", rev.FileWrite.State)
	}
	if rev.FileWrite.Content != prior {
		t.Errorf("Content mismatch; want %q, got %q", prior, rev.FileWrite.Content)
	}
	if rev.FileWrite.Mode != "0600" {
		t.Errorf("Mode = %s, want 0600", rev.FileWrite.Mode)
	}
	if rev.FileWrite.Owner != "deploy" {
		t.Errorf("Owner = %s, want deploy", rev.FileWrite.Owner)
	}
	if !rev.FileWrite.Force {
		t.Error("Force must be true on reverse")
	}
}

func TestReverse_ContentRoundTripsViaPriorContentBytes(t *testing.T) {
	// readAuthorizedKeys strips trailing '\n'; priorContentBytes
	// must re-add it.
	got := priorContentBytes([]string{"a", "b"}, true)
	if got != "a\nb\n" {
		t.Errorf("priorContentBytes = %q, want %q", got, "a\nb\n")
	}
	if priorContentBytes(nil, false) != "" {
		t.Error("empty bytes expected when file didn't exist")
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	step, err := h.Reverse(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "x"}}, r)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", step)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = "wrong"
	_, err := h.Reverse(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "x"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
