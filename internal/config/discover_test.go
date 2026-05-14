package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverConfig_PrimaryFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mooncake.yml")
	if err := os.WriteFile(target, []byte("- name: hi\n  shell: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverConfig: %v", err)
	}
	want, _ := filepath.Abs(target)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDiscoverConfig_MultiFileLayout(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "mooncake")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sub, "main.yml")
	if err := os.WriteFile(target, []byte("- name: hi\n  shell: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverConfig: %v", err)
	}
	want, _ := filepath.Abs(target)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDiscoverConfig_BothPresent_PrimaryWins(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "mooncake.yml")
	if err := os.WriteFile(primary, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(dir, "mooncake")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.yml"), []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverConfig: %v", err)
	}
	want, _ := filepath.Abs(primary)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDiscoverConfig_DirectoryNamedLikePrimary_Skipped(t *testing.T) {
	dir := t.TempDir()
	// Pathological: a directory literally named "mooncake.yml" — skip it
	// and fall through to mooncake/main.yml.
	if err := os.Mkdir(filepath.Join(dir, "mooncake.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "mooncake")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sub, "main.yml")
	if err := os.WriteFile(target, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverConfig: %v", err)
	}
	want, _ := filepath.Abs(target)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDiscoverConfig_NothingFound(t *testing.T) {
	dir := t.TempDir()
	_, err := DiscoverConfig(dir)
	var nfe *ErrNoConfigFound
	if !errors.As(err, &nfe) {
		t.Fatalf("want *ErrNoConfigFound, got %T: %v", err, err)
	}
	if !strings.Contains(nfe.Dir, filepath.Base(dir)) {
		t.Errorf("ErrNoConfigFound.Dir = %q, want it to contain temp dir basename %q", nfe.Dir, filepath.Base(dir))
	}
	if !filepath.IsAbs(nfe.Dir) {
		t.Errorf("ErrNoConfigFound.Dir = %q, want absolute path", nfe.Dir)
	}
}

func TestDiscoverConfig_SymlinkToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real.yml")
	if err := os.WriteFile(real, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "mooncake.yml")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverConfig: %v", err)
	}
	// os.Stat follows symlinks, so the returned path is the symlink itself
	// (the candidate we joined); resolution is left to the caller.
	wantPath, _ := filepath.Abs(link)
	if got != wantPath {
		t.Errorf("got %q want %q", got, wantPath)
	}
}

func TestHintNoConfigFound_ContainsAllParts(t *testing.T) {
	hint := HintNoConfigFound(&ErrNoConfigFound{Dir: "/tmp/empty"}, "apply")
	for _, want := range []string{
		"/tmp/empty",
		"mooncake.yml",
		"mooncake/main.yml",
		"mooncake init",
		"mooncake apply -c <path>",
	} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint missing %q\nfull hint:\n%s", want, hint)
		}
	}
}

func TestHintNoConfigFound_UsesGivenCmdName(t *testing.T) {
	for _, name := range []string{"apply", "plan", "validate"} {
		hint := HintNoConfigFound(&ErrNoConfigFound{Dir: "/x"}, name)
		want := "mooncake " + name + " -c <path>"
		if !strings.Contains(hint, want) {
			t.Errorf("cmd %q: hint missing %q", name, want)
		}
	}
}
