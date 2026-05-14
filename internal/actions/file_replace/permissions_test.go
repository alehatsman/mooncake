package file_replace

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_TableDriven exercises the Sudo×Path matrix for
// text.replace. Same structure as the file-family tests — relies on
// the shared actions.PathNeedsSudo helper, so a regression in the
// canonical system-roots list trips here.
func TestPermissions_TableDriven(t *testing.T) {
	h := &Handler{}
	tests := []struct {
		name      string
		path      string
		wantSudo  bool
		wantWrite []string
	}{
		{"system path /etc", "/etc/nginx/nginx.conf", true, []string{"/etc/nginx/nginx.conf"}},
		{"system path /var", "/var/lib/myapp/conf.yml", true, []string{"/var/lib/myapp/conf.yml"}},
		{"user path", "/home/user/.bashrc", false, []string{"/home/user/.bashrc"}},
		{"template prefix not flagged", "{{ etc_dir }}/foo", false, []string{"{{ etc_dir }}/foo"}},
		{"empty path", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{TextReplace: &config.FileReplace{Path: tt.path, Pattern: "x", Replace: "y"}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Sudo = %v, want %v", perms.Sudo, tt.wantSudo)
			}
			if perms.Network {
				t.Error("Network must be false for text.replace")
			}
			if !slicesEqual(perms.FilesystemWrite, tt.wantWrite) {
				t.Errorf("FilesystemWrite = %v, want %v", perms.FilesystemWrite, tt.wantWrite)
			}
		})
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
