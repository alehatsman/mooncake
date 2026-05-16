package fetch

import "testing"

func TestArchiveSuffix(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://example.com/x-1.0.tar.gz", ".tar.gz"},
		{"https://example.com/x-1.0.TGZ", ".tgz"},
		{"https://example.com/x.tar", ".tar"},
		{"https://example.com/x.zip", ".zip"},
		{"https://example.com/jq-linux-amd64", ".bin"},
		{"", ".bin"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.url, func(t *testing.T) {
			if got := archiveSuffix(c.url); got != c.want {
				t.Errorf("archiveSuffix(%q) = %q, want %q", c.url, got, c.want)
			}
		})
	}
}

func TestChecksumsEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"abc123", "abc123", true},
		{"sha256:abc123", "abc123", true},
		{"sha256:ABC123", "abc123", true},
		{"sha256:abc", "sha256:def", false},
		{"", "", true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.a+"_vs_"+c.b, func(t *testing.T) {
			if got := ChecksumsEqual(c.a, c.b); got != c.want {
				t.Errorf("ChecksumsEqual(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestNormalizeChecksum(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  sha256:ABC  ", "abc"},
		{"SHA256:abc", "abc"},
		{"abc", "abc"},
		{"", ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			if got := normalizeChecksum(c.in); got != c.want {
				t.Errorf("normalizeChecksum(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
