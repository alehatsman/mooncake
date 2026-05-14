package download

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_TableDriven covers the Sudo×Dest matrix. Network is
// always true (download fetches a URL); locked in by a dedicated test
// below so it can't drift.
func TestPermissions_TableDriven(t *testing.T) {
	h := Handler{}
	tests := []struct {
		name      string
		dest      string
		wantSudo  bool
		wantWrite []string
	}{
		{"system dest", "/usr/local/bin/mooncake", true, []string{"/usr/local/bin/mooncake"}},
		{"user dest", "/home/user/.local/bin/foo", false, []string{"/home/user/.local/bin/foo"}},
		{"empty dest", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{FileDownload: &config.Download{URL: "https://example.com/x", Dest: tt.dest}}
			perms := h.Permissions(step)
			if perms.Sudo != tt.wantSudo {
				t.Errorf("Sudo = %v, want %v", perms.Sudo, tt.wantSudo)
			}
			if !perms.Network {
				t.Error("Network must be true for file.download")
			}
			if !slicesEqual(perms.FilesystemWrite, tt.wantWrite) {
				t.Errorf("FilesystemWrite = %v, want %v", perms.FilesystemWrite, tt.wantWrite)
			}
		})
	}
}

// TestPermissions_NetworkAlwaysTrue locks in the spec-22 contract for
// file.download: Network=true regardless of Dest. If anyone ever
// removes the unconditional Network flag, this test catches it before
// the policy layer (later spec) starts approving network-bound steps
// it shouldn't.
func TestPermissions_NetworkAlwaysTrue(t *testing.T) {
	h := Handler{}
	for _, dest := range []string{"/etc/foo", "/tmp/bar", "/home/x/y", ""} {
		t.Run("dest="+dest, func(t *testing.T) {
			step := &config.Step{FileDownload: &config.Download{Dest: dest}}
			if !h.Permissions(step).Network {
				t.Errorf("Network=false for dest=%q; must always be true for file.download", dest)
			}
		})
	}
	// Also: nil step / nil FileDownload still surface Network=true,
	// because the action *type* is the indicator, not the value.
	if !h.Permissions(nil).Network {
		t.Error("Network=false for nil step; must be true (action type is the indicator)")
	}
}

func TestPermissions_NilHandled(t *testing.T) {
	h := Handler{}
	if got := h.Permissions(nil); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(nil) = %+v, want zero except Network", got)
	}
	if got := h.Permissions(&config.Step{}); got.Sudo || len(got.FilesystemWrite) != 0 {
		t.Errorf("Permissions(empty step) = %+v, want zero except Network", got)
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
