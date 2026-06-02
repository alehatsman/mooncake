package archive

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIsArchive(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// archives — recognised
		{"foo.tar.gz", true},
		{"FOO.TGZ", true}, // case-insensitive
		{"/abs/path/bundle.tar", true},
		{"release-v1.2.3.zip", true},

		// bare binaries — not archives
		{"jq-linux-amd64", false},
		{"/tmp/hadolint-Linux-x86_64", false},
		{"kubectl", false},

		// degenerate inputs
		{"", false},
		{"archive-without-extension", false},
		{"contains.tar.gz.txt", false}, // extension is .txt
	}
	for _, c := range cases {
		c := c
		t.Run(c.path, func(t *testing.T) {
			if got := IsArchive(c.path); got != c.want {
				t.Errorf("IsArchive(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestStripPath(t *testing.T) {
	cases := []struct {
		name  string
		strip int
		want  string
		ok    bool
	}{
		{"foo/bar.txt", 0, "foo/bar.txt", true},
		{"foo/bar.txt", 1, "bar.txt", true},
		{"top/sub/file.txt", 2, "file.txt", true},
		{"foo/bar.txt", 2, "", false}, // not enough components
		{".", 0, "", false},
		{"", 0, "", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got, ok := stripPath(c.name, c.strip)
			if ok != c.ok {
				t.Errorf("stripPath(%q, %d) ok = %v, want %v", c.name, c.strip, ok, c.ok)
			}
			if ok && got != c.want {
				t.Errorf("stripPath(%q, %d) = %q, want %q", c.name, c.strip, got, c.want)
			}
		})
	}
}

func TestSafeJoin_RejectsTraversal(t *testing.T) {
	if _, err := safeJoin("/tmp/dest", "../etc/passwd"); err == nil {
		t.Fatal("safeJoin should reject .. traversal")
	}
}

func TestCheckSymlinkTarget(t *testing.T) {
	dest := "/tmp/dest"
	cases := []struct {
		name     string
		linkPath string
		linkName string
		ok       bool
	}{
		{"in-dir relative", filepath.Join(dest, "bin", "ln"), "tool", true},
		{"absolute escape", filepath.Join(dest, "x"), "/etc", false},
		{"relative escape", filepath.Join(dest, "x"), "../../etc", false},
		{"relative within", filepath.Join(dest, "a", "b", "ln"), "../c", true},
		{"absolute within", filepath.Join(dest, "x"), filepath.Join(dest, "sub"), true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := checkSymlinkTarget(dest, c.linkPath, c.linkName)
			if (err == nil) != c.ok {
				t.Fatalf("checkSymlinkTarget(%q, %q, %q) err=%v, want ok=%v", dest, c.linkPath, c.linkName, err, c.ok)
			}
		})
	}
}

// TestExtractTarStream_SymlinkTraversal proves the classic symlink+file
// path-traversal attack is rejected: a symlink 'evil' -> '/<tmp>/outside'
// followed by a regular file 'evil/pwned' must NOT write through the link.
func TestExtractTarStream_SymlinkTraversal(t *testing.T) {
	root := t.TempDir()
	destDir := filepath.Join(root, "dest")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	// Symlink entry pointing outside destDir.
	if err := tw.WriteHeader(&tar.Header{
		Name:     "evil",
		Typeflag: tar.TypeSymlink,
		Linkname: outside,
		Mode:     0o777,
	}); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()

	err := extractTarStream(&buf, destDir, 0)
	if err == nil {
		t.Fatal("expected extractTarStream to reject escaping symlink, got nil")
	}
	if _, statErr := os.Lstat(filepath.Join(destDir, "evil")); statErr == nil {
		t.Fatal("escaping symlink was created on disk; it should have been rejected")
	}
}
