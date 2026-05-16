//nolint:revive // package name follows action convention
package os_ssh_key

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

const (
	key1 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO5kOBJpHCG5BvJ7s1abYbVjVTbb1k5L6XQpw2HmcZsR laptop"
	key2 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH7Z9aD5rJ8RHWXBjcOoiZ5KbsfvK6vXVQjYNu1u5cQ3 yubikey"
	key3 = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC4qY6q3K6 stale"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

func currentUsername(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	return u.Username
}

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"no user", &config.Step{OsSSHKey: &config.OsSSHKey{Key: key1}}, true},
		{"no keys", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u"}}, true},
		{"both forms", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Key: key1, Keys: []string{key2}}}, true},
		{"bad key", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Key: "not a real key"}}, true},
		{"bad state", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Key: key1, State: "wibble"}}, true},
		{"exclusive without keys", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Key: key1, Exclusive: true}}, true},
		{"ok single", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Key: key1}}, false},
		{"ok multi", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Keys: []string{key1, key2}}}, false},
		{"ok absent single", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Key: key1, State: "absent"}}, false},
		{"ok exclusive", &config.Step{OsSSHKey: &config.OsSSHKey{User: "u", Keys: []string{key1}, Exclusive: true}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestParseKey_IdentityIgnoresComment(t *testing.T) {
	a, err := parseKey(key1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := parseKey(strings.TrimSuffix(key1, " laptop") + " desktop")
	if err != nil {
		t.Fatal(err)
	}
	if a.identity() != b.identity() {
		t.Errorf("identity should ignore comment\n a=%s\n b=%s", a.identity(), b.identity())
	}
	if a.comment == b.comment {
		t.Errorf("comments should differ")
	}
}

func TestParseKey_WithOptions(t *testing.T) {
	pk, err := parseKey(`no-port-forwarding,no-X11-forwarding ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO5kOBJpHCG5BvJ7s1abYbVjVTbb1k5L6XQpw2HmcZsR me`)
	if err != nil {
		t.Fatal(err)
	}
	if pk.algo != "ssh-ed25519" {
		t.Errorf("algo=%s", pk.algo)
	}
	if len(pk.options) != 1 || pk.options[0] != "no-port-forwarding,no-X11-forwarding" {
		t.Errorf("options=%v", pk.options)
	}
}

func TestPresent_AddsKeyAndCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	r := mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: currentUsername(t),
		Key:  key1,
		Path: path,
	}})
	if !r.Changed {
		t.Fatal("expected change on first add")
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "AAAAC3NzaC1lZDI1NTE5AAAAIO5kOBJpHCG5BvJ7s1abYbVjVTbb1k5L6XQpw2HmcZsR") {
		t.Errorf("expected key in file; got %q", got)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected mode 0600; got %o", info.Mode().Perm())
	}
}

func TestPresent_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	step := &config.Step{OsSSHKey: &config.OsSSHKey{
		User: currentUsername(t),
		Key:  key1,
		Path: path,
	}}
	_ = mustRun(t, false, step)
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
}

func TestPresent_UpdateOptionsTriggersChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	// Pre-seed with the same key but different options.
	if err := os.WriteFile(path, []byte("from=10.0.0.0/8 "+key1+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User:    currentUsername(t),
		Key:     key1,
		Options: []string{"no-port-forwarding"},
		Path:    path,
	}})
	if !r.Changed {
		t.Fatal("expected change on options drift")
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "no-port-forwarding") {
		t.Errorf("expected new options; got %q", got)
	}
	if strings.Contains(string(got), "from=10.0.0.0/8") {
		t.Errorf("old options should be replaced; got %q", got)
	}
}

func TestExclusive_RemovesStaleKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(path, []byte(key1+"\n"+key3+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User:      currentUsername(t),
		Keys:      []string{key1, key2},
		Exclusive: true,
		Path:      path,
	}})
	if !r.Changed {
		t.Fatal("expected change")
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "AAAAC3NzaC1lZDI1NTE5AAAAIH7Z9aD5rJ8RHWXBjcOoiZ5KbsfvK6vXVQjYNu1u5cQ3") {
		t.Errorf("expected key2 in file; got %q", got)
	}
	if strings.Contains(string(got), "AAAAB3NzaC1yc2EAAAADAQABAAABAQC4qY6q3K6") {
		t.Errorf("stale key3 should be removed; got %q", got)
	}
}

func TestAbsent_RemovesKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(path, []byte(key1+"\n"+key2+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User:  currentUsername(t),
		Key:   key1,
		State: "absent",
		Path:  path,
	}})
	if !r.Changed {
		t.Fatal("expected change")
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "AAAAC3NzaC1lZDI1NTE5AAAAIO5kOBJpHCG5BvJ7s1abYbVjVTbb1k5L6XQpw2HmcZsR") {
		t.Errorf("key1 should be removed; got %q", got)
	}
	if !strings.Contains(string(got), "AAAAC3NzaC1lZDI1NTE5AAAAIH7Z9aD5rJ8RHWXBjcOoiZ5KbsfvK6vXVQjYNu1u5cQ3") {
		t.Errorf("key2 should remain; got %q", got)
	}
}

func TestAbsent_FileMissing_NoOp(t *testing.T) {
	r := mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User:  currentUsername(t),
		Key:   key1,
		State: "absent",
		Path:  filepath.Join(t.TempDir(), "authorized_keys"),
	}})
	if r.Changed {
		t.Errorf("absent on missing file should be noop; reason=%q", r.Reason)
	}
}

func TestPlan_DoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	r := mustRun(t, true, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: currentUsername(t),
		Key:  key1,
		Path: path,
	}})
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("plan must not write file")
	}
}

func TestPreservesCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	original := "# header comment\n\n" + key1 + "\n# trailer\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	// Add a new key — comments and blanks should survive.
	_ = mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: currentUsername(t),
		Key:  key2,
		Path: path,
	}})
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "# header comment") {
		t.Errorf("header comment must be preserved; got %q", got)
	}
	if !strings.Contains(string(got), "# trailer") {
		t.Errorf("trailer comment must be preserved; got %q", got)
	}
}

// F035 regression — Path 1. user.Lookup failure must surface as a
// loud error and prevent the write, instead of writing with uid=-1
// and leaving a file with caller ownership that sshd will refuse.
func TestRun_UserLookupFailureRefusesWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	step := &config.Step{OsSSHKey: &config.OsSSHKey{
		User: "no_such_user_F035_regression_xyz",
		Key:  key1,
		Path: path,
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected user-lookup error; got nil")
	}
	if !strings.Contains(err.Error(), "cannot determine uid/gid") {
		t.Errorf("error should mention uid/gid lookup; got %q", err.Error())
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("authorized_keys must NOT be written on lookup failure; stat err=%v", statErr)
	}
}

// F035 regression — Path 2. Chown EPERM (non-root caller trying to
// chown a file to another user) must surface as an error with a
// remediation hint, not be silently swallowed. Pre-fix the write
// returned success and the file was left with caller ownership.
func TestRun_ChownFailureSurfaces(t *testing.T) {
	originalChown := chownFn
	chownFn = func(string, int, int) error { return fs.ErrPermission }
	t.Cleanup(func() { chownFn = originalChown })

	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	step := &config.Step{OsSSHKey: &config.OsSSHKey{
		User: currentUsername(t),
		Key:  key1,
		Path: path,
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected chown EPERM to surface; got nil")
	}
	if !strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "become") {
		t.Errorf("error should hint at sudo/become remediation; got %q", err.Error())
	}
}

// F035 regression — Path 3. A pre-existing .ssh directory at a wider
// mode (0o755) must be tightened to 0o700 on a Run, because sshd's
// strict-mode check rejects authorized_keys under group/world-writable
// parents. Pre-fix the Chmod only ran for newly-created dirs.
func TestRun_TightensExistingSshDirMode(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Force mode after MkdirAll in case umask narrowed it.
	if err := os.Chmod(sshDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "authorized_keys")
	_ = mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: currentUsername(t),
		Key:  key1,
		Path: path,
	}})
	info, err := os.Stat(sshDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("expected .ssh dir tightened to 0700; got %o", info.Mode().Perm())
	}
}
