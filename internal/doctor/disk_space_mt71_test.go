package doctor

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckDiskSpace_HomeDirMissing is a regression test for MT-71.
// When ~/.mooncake/ doesn't exist yet (fresh install), the disk-space
// check used to report "unsupported on this OS" because statfs
// returned ENOENT. The fix walks up to an existing ancestor, so the
// probe now succeeds and the check returns a real free-space figure.
func TestCheckDiskSpace_HomeDirMissing(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, ".mooncake")

	r := checkDiskSpace{}.Run(Context{HomeDir: missing})

	if strings.Contains(r.Message, "unsupported on this OS") {
		t.Fatalf("MT-71 regression: disk-space reported unsupported when %s didn't exist:\n  status=%v message=%q",
			missing, r.Status, r.Message)
	}
	if r.Status != StatusOK && r.Status != StatusWarning {
		t.Errorf("expected StatusOK or StatusWarning, got %v (message=%q)", r.Status, r.Message)
	}
	// The message should reference an existing ancestor path (tmp itself or higher).
	if !strings.Contains(r.Message, "GiB free") {
		t.Errorf("expected message to mention free space, got %q", r.Message)
	}
}

func TestExistingAncestor(t *testing.T) {
	tmp := t.TempDir()

	cases := []struct {
		name string
		dir  string
		want string
	}{
		{"existing dir", tmp, tmp},
		{"missing leaf", filepath.Join(tmp, "leaf"), tmp},
		{"missing two-level", filepath.Join(tmp, "a", "b"), tmp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := existingAncestor(tc.dir)
			if err != nil {
				t.Fatalf("existingAncestor(%q) error: %v", tc.dir, err)
			}
			if got != tc.want {
				t.Errorf("existingAncestor(%q) = %q, want %q", tc.dir, got, tc.want)
			}
		})
	}
}
