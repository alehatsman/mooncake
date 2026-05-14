package file_patch_apply

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_TableDriven exercises text.patch's Sudo×Path matrix.
// PatchFile is intentionally NOT included in the write set even though
// it's read at apply time — only Path is mutated. See
// TestPermissions_PatchFileNotInWriteSet below.
func TestPermissions_TableDriven(t *testing.T) {
	h := &Handler{}
	tests := []struct {
		name      string
		path      string
		wantSudo  bool
		wantWrite []string
	}{
		{"system /etc", "/etc/apt/sources.list.d/x.list", true, []string{"/etc/apt/sources.list.d/x.list"}},
		{"system /var", "/var/lib/dpkg/status", true, []string{"/var/lib/dpkg/status"}},
		{"user", "/home/user/notes.md", false, []string{"/home/user/notes.md"}},
		{"templated", "{{ home }}/notes.md", false, []string{"{{ home }}/notes.md"}},
		{"empty", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{TextPatch: &config.FilePatchApply{
				Path:  tt.path,
				Patch: "--- a\n+++ b\n@@\n-x\n+y\n",
			}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Sudo = %v, want %v", perms.Sudo, tt.wantSudo)
			}
			if perms.Network {
				t.Error("Network must be false for text.patch")
			}
			if !slicesEqual(perms.FilesystemWrite, tt.wantWrite) {
				t.Errorf("FilesystemWrite = %v, want %v", perms.FilesystemWrite, tt.wantWrite)
			}
		})
	}
}

// TestPermissions_PatchFileNotInWriteSet locks in the spec-22
// contract for text.patch: PatchFile is a READ-ONLY input on the
// controller's filesystem. It must never appear in FilesystemWrite,
// or a future policy layer would mistakenly require write
// permissions on patches.
func TestPermissions_PatchFileNotInWriteSet(t *testing.T) {
	h := &Handler{}
	step := &config.Step{TextPatch: &config.FilePatchApply{
		Path:      "/etc/target",
		PatchFile: "/etc/patches/x.patch", // read-only input
	}}
	perms := h.Permissions(step)
	if len(perms.FilesystemWrite) != 1 || perms.FilesystemWrite[0] != "/etc/target" {
		t.Errorf("FilesystemWrite = %v, want exactly [/etc/target] (PatchFile must NOT be included)", perms.FilesystemWrite)
	}
}

func TestPermissions_NilHandled(t *testing.T) {
	h := &Handler{}
	if got := h.Permissions(nil); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(nil) = %+v, want zero", got)
	}
	if got := h.Permissions(&config.Step{}); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(empty step) = %+v, want zero", got)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	if !actions.IsPermitter(&Handler{}) {
		t.Error("*Handler should satisfy actions.Permitter")
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
