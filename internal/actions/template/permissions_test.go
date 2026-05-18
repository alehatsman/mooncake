package template

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_TableDriven mirrors file.write's test surface so any
// regression that breaks the shared actions.PathNeedsSudo helper trips
// in every file-family handler at once. Each entry exercises a different
// branch: literal system root, user-space, template, empty.
func TestPermissions_TableDriven(t *testing.T) {
	h := Handler{}
	tests := []struct {
		name      string
		dest      string
		wantSudo  bool
		wantWrite []string
	}{
		{"system dest /etc", "/etc/nginx/nginx.conf", true, []string{"/etc/nginx/nginx.conf"}},
		{"system dest /usr/local", "/usr/local/bin/foo.sh", true, []string{"/usr/local/bin/foo.sh"}},
		{"user dest", "/home/user/.zshrc", false, []string{"/home/user/.zshrc"}},
		{"template prefix not flagged", "{{ etc_dir }}/foo", false, []string{"{{ etc_dir }}/foo"}},
		{"empty dest produces zero PermissionSet", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{FileTemplate: &config.Template{Dest: tt.dest}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Sudo = %v, want %v", perms.Sudo, tt.wantSudo)
			}
			if perms.Network {
				t.Error("Network should be false for file.template (purely local-FS)")
			}
			if !slicesEqual(perms.FilesystemWrite, tt.wantWrite) {
				t.Errorf("FilesystemWrite = %v, want %v", perms.FilesystemWrite, tt.wantWrite)
			}
		})
	}
}

// TestPermissions_NilHandled covers defensive nil paths so accidentally
// constructed steps in tests/dry-runs don't panic.
func TestPermissions_NilHandled(t *testing.T) {
	h := Handler{}
	if got := h.Permissions(nil); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(nil) = %+v, want zero", got)
	}
	if got := h.Permissions(&config.Step{}); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(empty step) = %+v, want zero", got)
	}
}

// TestPermissions_RegisteredAsPermitter — must catch a future refactor
// that accidentally narrows the receiver and breaks the interface.
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
