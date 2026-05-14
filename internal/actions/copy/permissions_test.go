package copy

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_TableDriven exercises Sudo + FilesystemWrite for the
// file.copy handler. Same shape as file.template; what matters for
// file.copy specifically is that Src is NEVER used as the FS-write
// target (only Dest is written), so a copy from /etc/ to /tmp/ must
// not request Sudo.
func TestPermissions_TableDriven(t *testing.T) {
	h := Handler{}
	tests := []struct {
		name      string
		src       string
		dest      string
		wantSudo  bool
		wantWrite []string
	}{
		{"system dest", "/tmp/src", "/etc/hosts", true, []string{"/etc/hosts"}},
		{"user dest from system src", "/etc/hosts.template", "/home/user/hosts.bak", false, []string{"/home/user/hosts.bak"}},
		{"user-to-user", "/home/user/a", "/home/user/b", false, []string{"/home/user/b"}},
		{"empty dest", "/tmp/src", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{FileCopy: &config.Copy{Src: tt.src, Dest: tt.dest}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Sudo = %v, want %v", perms.Sudo, tt.wantSudo)
			}
			if perms.Network {
				t.Error("Network must be false for file.copy")
			}
			if !slicesEqual(perms.FilesystemWrite, tt.wantWrite) {
				t.Errorf("FilesystemWrite = %v, want %v", perms.FilesystemWrite, tt.wantWrite)
			}
		})
	}
}

func TestPermissions_NilHandled(t *testing.T) {
	h := Handler{}
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
