package os_ssh_key //nolint:revive // package name follows action convention

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
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
	after, ok := d.After.(*OsSSHKeySnapshot)
	if !ok {
		t.Fatalf("After is not *OsSSHKeySnapshot")
	}
	if after.KeyCount != 3 {
		t.Errorf("KeyCount = %d, want 3", after.KeyCount)
	}
}

func TestDiff_DoesNotLeakKeyMaterial(t *testing.T) {
	h := Handler{}
	secret := "ssh-ed25519 SUPERSECRET_PUB_KEY"
	d, _ := h.Diff(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: secret}})
	after := d.After.(*OsSSHKeySnapshot)
	// The snapshot must NOT include the key material itself — it
	// only carries count + metadata. (Public keys aren't strictly
	// secret, but Diff is a structural surface that may be logged
	// or shipped to agents; keeping key material off it is the
	// conservative choice.)
	body := after.User + after.State + after.Path
	if strings.Contains(body, secret) {
		t.Errorf("OsSSHKeySnapshot must not contain key material; found: %s", body)
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

func TestReverse_RefusesPendingCapture(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{OsSSHKey: &config.OsSSHKey{User: "deploy", Key: "x"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
