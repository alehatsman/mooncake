package tool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions/tool/archive"
)

// TestMT40_InstallBareBinary is a regression test for manual-test #40
// (2026-05-15): the github-release / archive-url install pipeline used
// to call extractArchive unconditionally, which failed with
// "unsupported archive format" when the released asset was a bare
// binary (jq, hadolint, kind, etc.). After this fix the pipeline
// routes such artifacts through installBareBinary instead.
func TestMT40_InstallBareBinary(t *testing.T) {
	tmpDir := t.TempDir()
	// Simulate a downloaded binary by writing some bytes to a file
	// with no recognized archive extension.
	src := filepath.Join(tmpDir, "mooncake-tool-1234567.bin")
	if err := os.WriteFile(src, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmpDir, "install")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	spec := Spec{Name: "jq", Version: "1.7.1"}
	plan := Plan{}
	if err := installBareBinary(src, destDir, spec, plan); err != nil {
		t.Fatalf("installBareBinary: %v", err)
	}

	// Should land at destDir/jq with 0o755 + executable bit.
	dest := filepath.Join(destDir, "jq")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o755 {
		t.Errorf("dest mode = %o, want 0755", mode)
	}
	if info.Size() == 0 {
		t.Errorf("dest is empty")
	}
}

func TestMT40_BareBinaryName_Precedence(t *testing.T) {
	cases := []struct {
		name string
		spec Spec
		plan Plan
		want string
	}{
		{"explicit Bin wins", Spec{Name: "jq", Bin: "/usr/local/bin/jq-1.7"}, Plan{BinRel: "ignored"}, "jq-1.7"},
		{"falls back to plan.BinRel", Spec{Name: "kind"}, Plan{BinRel: "bin/kind"}, "kind"},
		{"falls back to spec.Name", Spec{Name: "hadolint"}, Plan{}, "hadolint"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := bareBinaryName(c.spec, c.plan); got != c.want {
				t.Errorf("bareBinaryName = %q, want %q", got, c.want)
			}
		})
	}
}

// TestMT40_DetectFormat_UnknownForBareBinary confirms archive.IsArchive
// returns false for typical bare-binary filenames — which is the
// signal InstallURL uses to route to installBareBinary.
func TestMT40_DetectFormat_UnknownForBareBinary(t *testing.T) {
	cases := []string{
		"/tmp/mooncake-tool-1234.bin",
		"/tmp/jq-linux-amd64",
		"/tmp/kind-linux-amd64",
		"/tmp/hadolint-Linux-x86_64",
	}
	for _, c := range cases {
		c := c
		t.Run(c, func(t *testing.T) {
			if archive.IsArchive(c) {
				t.Errorf("archive.IsArchive(%q) = true, want false", c)
			}
		})
	}
}
