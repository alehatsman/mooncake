package tool

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/lockfile"
)

// buildTarGz returns a tar.gz archive containing the given files.
// Each file is created as a regular file under a single top-level
// "pkg/" directory so callers can exercise strip_components.
func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name:     "pkg/" + name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// withTempStore points the install dir under t.TempDir() via XDG_DATA_HOME
// so tests do not write to the user's real ~/.local/share/mooncake.
func withTempStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	return filepath.Join(dir, "mooncake", "tools")
}

func TestInstallURL_HappyPath_TOFU(t *testing.T) {
	storeRoot := withTempStore(t)

	archive := buildTarGz(t, map[string]string{"bin/go": "#!/bin/sh\necho go\n"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(archive); err != nil {
			t.Fatalf("server write: %v", err)
		}
	}))
	defer srv.Close()

	spec := Spec{
		Backend:         BackendArchiveURL,
		Name:            "go",
		Version:         "1.25.3",
		URL:             srv.URL + "/go.tar.gz",
		StripComponents: 1,
		Bin:             "bin/go",
	}
	plan := Plan{URL: srv.URL + "/go.tar.gz", StripComponents: 1, UseSharedPipeline: true}
	facts := FactSnapshot{OS: "linux", Arch: "amd64"}

	lock := &lockfile.Lock{}
	out, err := InstallURL(context.Background(), spec, plan, facts, lock)
	if err != nil {
		t.Fatalf("InstallURL: %v", err)
	}
	if !out.Changed {
		t.Error("expected Changed=true on first install")
	}
	wantDir := filepath.Join(storeRoot, "go", "1.25.3")
	if out.InstallDir != wantDir {
		t.Errorf("InstallDir = %q, want %q", out.InstallDir, wantDir)
	}
	// strip_components=1 means "pkg/bin/go" → "bin/go".
	binPath := filepath.Join(out.InstallDir, "bin", "go")
	if _, err := os.Stat(binPath); err != nil {
		t.Errorf("expected installed file at %s: %v", binPath, err)
	}

	// Lock entry recorded with computed checksum.
	e, ok := lock.Lookup(BackendArchiveURL, "go", "1.25.3", "linux-amd64")
	if !ok {
		t.Fatal("lock entry missing")
	}
	if e.SHA256 == "" {
		t.Error("expected SHA256 recorded after TOFU install")
	}
}

func TestInstallURL_Idempotent(t *testing.T) {
	withTempStore(t)

	archive := buildTarGz(t, map[string]string{"bin/go": "x"})
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write(archive)
	}))
	defer srv.Close()

	spec := Spec{Backend: BackendArchiveURL, Name: "go", Version: "1.25.3", StripComponents: 1}
	plan := Plan{URL: srv.URL + "/go.tar.gz", StripComponents: 1, UseSharedPipeline: true}
	facts := FactSnapshot{OS: "linux", Arch: "amd64"}

	lock := &lockfile.Lock{}
	if _, err := InstallURL(context.Background(), spec, plan, facts, lock); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if hits != 1 {
		t.Errorf("expected 1 HTTP hit, got %d", hits)
	}

	out, err := InstallURL(context.Background(), spec, plan, facts, lock)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if out.Changed {
		t.Error("expected Changed=false on second install")
	}
	if hits != 1 {
		t.Errorf("expected NO additional HTTP hits, got %d total", hits)
	}
}

func TestInstallURL_ChecksumMismatchFails(t *testing.T) {
	withTempStore(t)

	archive := buildTarGz(t, map[string]string{"bin/go": "real"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer srv.Close()

	spec := Spec{Backend: BackendArchiveURL, Name: "go", Version: "1.25.3"}
	plan := Plan{
		URL:               srv.URL + "/go.tar.gz",
		Checksum:          "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		StripComponents:   1,
		UseSharedPipeline: true,
	}
	_, err := InstallURL(context.Background(), spec, plan, FactSnapshot{OS: "linux", Arch: "amd64"}, &lockfile.Lock{})
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestInstallURL_LockChecksumEnforced(t *testing.T) {
	withTempStore(t)

	archive := buildTarGz(t, map[string]string{"bin/go": "x"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer srv.Close()

	spec := Spec{Backend: BackendArchiveURL, Name: "go", Version: "1.25.3"}
	plan := Plan{URL: srv.URL + "/go.tar.gz", StripComponents: 1, UseSharedPipeline: true}
	facts := FactSnapshot{OS: "linux", Arch: "amd64"}

	// Pre-seed a lock entry with the wrong checksum.
	lock := &lockfile.Lock{}
	lock.Set(lockfile.Entry{
		Backend:      BackendArchiveURL,
		Name:         "go",
		Version:      "1.25.3",
		SHA256:       "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		LockedByArch: "linux-amd64",
	})

	_, err := InstallURL(context.Background(), spec, plan, facts, lock)
	if err == nil {
		t.Fatal("expected lockfile checksum enforcement to fail the install")
	}
}
