package package_handler

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_NetworkAlways pkg always fetches from a remote
// repo, so Network must stay on regardless of resolved manager or
// step shape. Even degenerate inputs (nil step, empty Names,
// state=absent) keep the flag on so a policy layer can't be
// sidestepped by a malformed step.
func TestPermissions_NetworkAlways(t *testing.T) {
	h := &Handler{}
	cases := []*config.Step{
		nil,
		{},
		{Pkg: &config.Package{}},
		{Pkg: &config.Package{Name: "vim"}},
		{Pkg: &config.Package{Name: "vim", State: "absent"}},
		{Pkg: &config.Package{Names: []string{"a", "b", "c"}}},
		{Pkg: &config.Package{Manager: "yay", Names: []string{"git-delta"}}},
		{Pkg: &config.Package{Manager: "brew", Names: []string{"jq"}}},
	}
	for i, step := range cases {
		got := h.Permissions(step)
		if !got.Network {
			t.Errorf("case %d: Network = false, must be true regardless of step shape", i)
		}
	}
}

// TestPermissions_SudoByManager — F049: pkg.Permissions must read
// step.Pkg.Manager and refuse to declare Sudo:true for managers that
// refuse root by design (yay/paru — AUR wrappers; brew — user-prefix
// installer). Spec-72 follow-up: the auto-detect (empty manager)
// case is covered separately by TestPermissions_SudoByManager_AutoDetect
// because the resolved manager depends on what's on the test host's
// PATH.
func TestPermissions_SudoByManager(t *testing.T) {
	h := &Handler{}
	cases := []struct {
		manager  string
		wantSudo bool
	}{
		{"apt", true},    // dpkg-based, writes /var/lib/dpkg
		{"dnf", true},    // rpm-based
		{"yum", true},    // rpm-based, RHEL 7
		{"pacman", true}, // system manager on Arch
		{"yay", false},   // AUR wrapper, refuses root
		{"paru", false},  // AUR wrapper, refuses root
		{"brew", false},  // user-prefix install
		{"choco", true},  // Windows; preflight already platform-gated
	}
	for _, c := range cases {
		t.Run(c.manager, func(t *testing.T) {
			step := &config.Step{Pkg: &config.Package{Manager: c.manager}}
			got := h.Permissions(step)
			if got.Sudo != c.wantSudo {
				t.Errorf("manager=%q: Sudo = %v, want %v", c.manager, got.Sudo, c.wantSudo)
			}
		})
	}
}

// TestPermissions_SudoByManager_AutoDetect — spec-72 follow-up:
// empty manager triggers determinePackageManager at preflight time,
// not at apply time, so a macOS-with-brew host gets Sudo:false at
// preflight and rejects `as_user: root` before the brew driver runs.
// This was the gap the F049 author punted on ("auto-detection
// resolves at apply-time"); Layer C makes it cheap to close because
// the preflight reason taxonomy lets operators see precisely why an
// auto-detect step was rejected.
//
// What we can assert here without mocking lookPath: whatever manager
// auto-detection resolved to on the test host must match the
// manager's Sudo policy. If the resolution failed entirely (no
// manager on PATH), Sudo stays true as the safer default.
func TestPermissions_SudoByManager_AutoDetect(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{}} // Manager intentionally empty
	got := h.Permissions(step)

	resolved, err := h.determinePackageManager("", nil)
	if err != nil {
		// No manager detected on PATH → Sudo:true is the documented
		// safer default; the apply will fail with a manager-detection
		// error inside Run regardless.
		if !got.Sudo {
			t.Errorf("Sudo = false, want true (no manager auto-detected, safer default)")
		}
		return
	}
	wantSudo := managerRequiresSudo(resolved)
	if got.Sudo != wantSudo {
		t.Errorf("auto-detected manager=%q: Sudo = %v, want %v", resolved, got.Sudo, wantSudo)
	}
}

// TestPermissions_NoFilesystemWriteOrBinaries — pkg deliberately
// leaves FilesystemWrite + RequiredBinaries empty. FilesystemWrite
// because installs go to system-managed paths that don't map to a
// step field. RequiredBinaries because the handler auto-detects the
// manager; pinning a specific binary on PATH would break detection.
// Locked in so a future refactor doesn't accidentally add a write
// target or a hard binary dep.
func TestPermissions_NoFilesystemWriteOrBinaries(t *testing.T) {
	h := &Handler{}
	got := h.Permissions(&config.Step{Pkg: &config.Package{Name: "vim"}})
	if len(got.FilesystemWrite) != 0 {
		t.Errorf("FilesystemWrite = %v, want empty (pkg uses system-managed paths)", got.FilesystemWrite)
	}
	if len(got.RequiredBinaries) != 0 {
		t.Errorf("RequiredBinaries = %v, want empty (handler auto-detects manager)", got.RequiredBinaries)
	}
}

// TestPermissions_RegisteredAsPermitter catches a future refactor
// that narrows the receiver style and accidentally breaks the
// interface.
func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	if !actions.IsPermitter(&Handler{}) {
		t.Error("*Handler should satisfy actions.Permitter")
	}
}
