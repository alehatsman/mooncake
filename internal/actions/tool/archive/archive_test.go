package archive

import "testing"

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
