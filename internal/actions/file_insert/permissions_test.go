package file_insert

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_TableDriven exercises text.insert's Permitter
// contract across system + user + templated + empty paths.
func TestPermissions_TableDriven(t *testing.T) {
	h := &Handler{}
	tests := []struct {
		name      string
		path      string
		wantSudo  bool
		wantWrite []string
	}{
		{"system /etc", "/etc/sudoers.d/myrule", true, []string{"/etc/sudoers.d/myrule"}},
		{"system /usr", "/usr/share/applications/x.desktop", true, []string{"/usr/share/applications/x.desktop"}},
		{"user", "/home/user/.config/app/x.ini", false, []string{"/home/user/.config/app/x.ini"}},
		{"templated", "{{ home }}/x", false, []string{"{{ home }}/x"}},
		{"empty", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{TextInsert: &config.FileInsert{
				Path:     tt.path,
				Anchor:   "FOO",
				Position: "after",
				Content:  "bar",
			}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Sudo = %v, want %v", perms.Sudo, tt.wantSudo)
			}
			if perms.Network {
				t.Error("Network must be false for text.insert")
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
