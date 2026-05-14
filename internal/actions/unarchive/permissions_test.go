package unarchive

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_TableDriven exercises the unarchive Permitter. Like
// file.copy, only Dest is the write target — Src is a read of an
// archive on the controller filesystem. A copy from /etc/archive.tar
// into /tmp/extracted must not flag Sudo.
func TestPermissions_TableDriven(t *testing.T) {
	h := Handler{}
	tests := []struct {
		name      string
		src       string
		dest      string
		wantSudo  bool
		wantWrite []string
	}{
		{"system dest", "/tmp/bundle.tar", "/usr/local/share/app", true, []string{"/usr/local/share/app"}},
		{"user dest from system src", "/etc/templates/x.tar", "/home/user/x", false, []string{"/home/user/x"}},
		{"empty dest", "/tmp/x.tar", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{FileUnarchive: &config.Unarchive{Src: tt.src, Dest: tt.dest}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Sudo = %v, want %v", perms.Sudo, tt.wantSudo)
			}
			if perms.Network {
				t.Error("Network must be false for file.unarchive (local-only)")
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
