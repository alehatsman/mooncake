package fleet

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// makeOverlayDir builds a temp plan-dir containing the named overlay files
// under vars/. Returns the plan-dir's absolute path. Each entry in files is
// a path relative to planDir/vars (e.g. "common.yml", "by-host/laptop.yml").
func makeOverlayDir(t *testing.T, files ...string) string {
	t.Helper()
	planDir := t.TempDir()
	for _, rel := range files {
		full := filepath.Join(planDir, "vars", rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte("# "+rel+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return planDir
}

// rel collapses absolute paths back to plan-dir-relative ones, with forward
// slashes, so test assertions stay portable across hosts.
func rel(t *testing.T, planDir string, paths []string) []string {
	t.Helper()
	out := make([]string, len(paths))
	for i, p := range paths {
		r, err := filepath.Rel(planDir, p)
		if err != nil {
			t.Fatalf("rel %s: %v", p, err)
		}
		out[i] = filepath.ToSlash(r)
	}
	return out
}

func TestResolveVarsFiles_AllOverlaysPresent(t *testing.T) {
	planDir := makeOverlayDir(t,
		"common.yml",
		"by-tag/darwin.yml",
		"by-tag/gpu.yml",
		"by-host/macbook.yml",
	)
	peer := Peer{Name: "macbook", Tags: []string{"darwin", "gpu"}}

	got := rel(t, planDir, ResolveVarsFiles(planDir, peer))
	want := []string{
		"vars/common.yml",
		"vars/by-tag/darwin.yml",
		"vars/by-tag/gpu.yml",
		"vars/by-host/macbook.yml",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestResolveVarsFiles_TagOrderMatchesPeer(t *testing.T) {
	// Tag-by-tag overlays must be loaded in the order tags appear in peer.Tags,
	// not in lexical or filesystem order. Asserts deterministic ordering.
	planDir := makeOverlayDir(t, "by-tag/alpha.yml", "by-tag/zebra.yml")
	peer := Peer{Name: "x", Tags: []string{"zebra", "alpha"}}

	got := rel(t, planDir, ResolveVarsFiles(planDir, peer))
	want := []string{"vars/by-tag/zebra.yml", "vars/by-tag/alpha.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tag order:\n got: %v\nwant: %v", got, want)
	}
}

func TestResolveVarsFiles_OnlyCommon(t *testing.T) {
	planDir := makeOverlayDir(t, "common.yml")
	peer := Peer{Name: "laptop", Tags: []string{"darwin"}}

	got := rel(t, planDir, ResolveVarsFiles(planDir, peer))
	want := []string{"vars/common.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveVarsFiles_OnlyByHost(t *testing.T) {
	planDir := makeOverlayDir(t, "by-host/laptop.yml")
	peer := Peer{Name: "laptop", Tags: []string{"darwin"}}

	got := rel(t, planDir, ResolveVarsFiles(planDir, peer))
	want := []string{"vars/by-host/laptop.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveVarsFiles_NoOverlays(t *testing.T) {
	planDir := t.TempDir()
	peer := Peer{Name: "laptop", Tags: []string{"darwin"}}

	got := ResolveVarsFiles(planDir, peer)
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestResolveVarsFiles_PeerWithoutTags(t *testing.T) {
	planDir := makeOverlayDir(t, "common.yml", "by-host/vps.yml")
	peer := Peer{Name: "vps", Tags: nil}

	got := rel(t, planDir, ResolveVarsFiles(planDir, peer))
	want := []string{"vars/common.yml", "vars/by-host/vps.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveVarsFiles_DirectoryAtCandidatePathIsSkipped(t *testing.T) {
	// Defensive: if someone creates `vars/common.yml/` as a directory by
	// accident, we should not return it as a vars file. Stat would succeed
	// but it's not loadable.
	planDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(planDir, "vars", "common.yml"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	peer := Peer{Name: "x"}

	got := ResolveVarsFiles(planDir, peer)
	if len(got) != 0 {
		t.Fatalf("expected empty (directory at candidate path), got %v", got)
	}
}
