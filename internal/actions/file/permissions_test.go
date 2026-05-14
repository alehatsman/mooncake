package file

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_SystemPath_DeclaresSudo locks in the file.write
// Permitter contract: writes under known system directories declare
// Sudo so the executor preflight can fail-fast before the write.
//
// Table-driven across the well-known POSIX system roots — if a new
// root is added to systemPathPrefixes, the maintainer should also
// extend this table.
func TestPermissions_SystemPath_DeclaresSudo(t *testing.T) {
	h := Handler{}
	tests := []struct {
		path     string
		wantSudo bool
	}{
		{"/etc/hosts", true},
		{"/etc/ssh/sshd_config", true},
		{"/usr/local/bin/foo", true},
		{"/var/log/app.log", true},
		{"/opt/myapp/conf.yml", true},
		{"/boot/grub.cfg", true},
		{"/root/.ssh/authorized_keys", true},
		{"/lib/systemd/system/x.service", true},
		{"/sbin/init-wrapper", true},
		{"/srv/data/index.html", true},

		// User-space paths — must NOT declare Sudo.
		{"/home/user/.zshrc", false},
		{"/tmp/scratch.txt", false},
		{"./relative.txt", false},
		{"relative.txt", false},

		// Template prefix — conservative: don't claim Sudo we can't
		// verify. Runtime EACCES is the backstop.
		{"{{ etc_dir }}/foo", false},

		// Empty path — gracefully handled, no Sudo.
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			step := &config.Step{FileWrite: &config.File{Path: tt.path}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Permissions(%q).Sudo = %v, want %v", tt.path, perms.Sudo, tt.wantSudo)
			}
		})
	}
}

// TestPermissions_FilesystemWritePopulated — the FilesystemWrite slice
// must echo the declared path even when Sudo is false, so a future
// policy layer can allowlist by path without re-parsing the step.
func TestPermissions_FilesystemWritePopulated(t *testing.T) {
	h := Handler{}
	step := &config.Step{FileWrite: &config.File{Path: "/home/user/.zshrc"}}
	perms := h.Permissions(step)
	if len(perms.FilesystemWrite) != 1 || perms.FilesystemWrite[0] != "/home/user/.zshrc" {
		t.Errorf("FilesystemWrite = %v, want [/home/user/.zshrc]", perms.FilesystemWrite)
	}
}

// TestPermissions_NoNetworkNoBinaries — file.write is purely local-FS.
// Lock in that Network and RequiredBinaries are NEVER populated;
// otherwise the wiring would be wrong for the wrong reason.
func TestPermissions_NoNetworkNoBinaries(t *testing.T) {
	h := Handler{}
	step := &config.Step{FileWrite: &config.File{Path: "/etc/hosts"}}
	perms := h.Permissions(step)
	if perms.Network {
		t.Error("file.write should never declare Network: true")
	}
	if len(perms.RequiredBinaries) != 0 {
		t.Errorf("file.write should declare no RequiredBinaries, got %v", perms.RequiredBinaries)
	}
}

// TestPermissions_NilStep_NilFileWrite_HandledGracefully covers the
// defensive paths in Permissions(). The contract on Permitter says
// "given any *config.Step, return a sane PermissionSet" — including
// degenerate inputs (mid-construction tests, mocks, etc.) shouldn't
// panic.
func TestPermissions_NilStep_NilFileWrite_HandledGracefully(t *testing.T) {
	h := Handler{}
	if got := h.Permissions(nil); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(nil) = %+v, want zero value", got)
	}
	step := &config.Step{} // FileWrite == nil
	if got := h.Permissions(step); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(step{FileWrite:nil}) = %+v, want zero value", got)
	}
}

// TestPermissions_IsRegisteredAsPermitter wires the Is* capability
// check from spec-22 against the real registered Handler. If anyone
// removes file.write's Permitter impl (or accidentally narrows the
// receiver from pointer-to-value-or-vice-versa breaking the
// interface), this test catches it.
func TestPermissions_IsRegisteredAsPermitter(t *testing.T) {
	// Use pointer receiver to match the registered handler in init():
	// Handler has pointer-receiver methods (e.g. Run), so only *Handler
	// satisfies actions.Handler. Permissions itself is a value-receiver
	// method which means both *Handler and Handler satisfy Permitter,
	// but we test the form that's actually registered.
	h := &Handler{}
	if !actions.IsPermitter(h) {
		t.Error("*Handler should be detected as a Permitter")
	}
}
