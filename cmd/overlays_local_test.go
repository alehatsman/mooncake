package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// runWithResolveLocalOverlays drives resolveLocalOverlays through a fake
// cli.App configured with the spec-51 flags. Returns the resolved overlay
// list (or an error) so test cases can assert on either.
func runWithResolveLocalOverlays(t *testing.T, configPath string, args ...string) ([]string, error) {
	t.Helper()
	var (
		got    []string
		gotErr error
	)
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "host"},
			&cli.StringFlag{Name: "overlays", Value: "on"},
		},
		Action: func(c *cli.Context) error {
			got, gotErr = resolveLocalOverlays(c, configPath)
			return nil
		},
	}
	full := append([]string{"test"}, args...)
	if err := app.Run(full); err != nil {
		t.Fatalf("app.Run: %v", err)
	}
	return got, gotErr
}

func writeOverlay(t *testing.T, planDir, rel string) string {
	t.Helper()
	full := filepath.Join(planDir, "vars", rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte("# "+rel+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return filepath.Clean(full)
}

func TestResolveLocalOverlays_DerivedHostnamePicksUpByHost(t *testing.T) {
	host, err := hostnameForLocalOverlays()
	if err != nil {
		t.Fatalf("hostnameForLocalOverlays: %v", err)
	}

	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	wantCommon := writeOverlay(t, planDir, "common.yml")
	wantByHost := writeOverlay(t, planDir, "by-host/"+host+".yml")

	got, gotErr := runWithResolveLocalOverlays(t, configPath)
	if gotErr != nil {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	want := []string{wantCommon, wantByHost}
	if !slicesEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestResolveLocalOverlays_DerivedHostnameSilentMiss(t *testing.T) {
	// No vars/ directory at all. Default flags (no --host). Must return
	// empty without error — spec-51 §G4 "silent on a clean miss".
	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, gotErr := runWithResolveLocalOverlays(t, configPath)
	if gotErr != nil {
		t.Fatalf("expected silent miss, got error: %v", gotErr)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty overlays, got %v", got)
	}
}

func TestResolveLocalOverlays_ExplicitHostFlagMissIsNoisy(t *testing.T) {
	// --host names a host whose by-host file doesn't exist. Spec-51 §G4
	// "noisy on an explicit miss".
	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, gotErr := runWithResolveLocalOverlays(t, configPath, "--host", "macbook")
	if gotErr == nil {
		t.Fatalf("expected error for explicit miss, got nil")
	}
	if !strings.Contains(gotErr.Error(), "macbook") || !strings.Contains(gotErr.Error(), "overlay file not found") {
		t.Fatalf("error %q does not mention host or the not-found context", gotErr)
	}
}

func TestResolveLocalOverlays_ExplicitHostFlagFindsFile(t *testing.T) {
	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	wantByHost := writeOverlay(t, planDir, "by-host/macbook.yml")

	got, gotErr := runWithResolveLocalOverlays(t, configPath, "--host", "macbook")
	if gotErr != nil {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	want := []string{wantByHost}
	if !slicesEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveLocalOverlays_OverlaysOffDisablesAutoLoad(t *testing.T) {
	// Even with the file present, --overlays=off returns nil. Spec-51 §G3.
	host, err := hostnameForLocalOverlays()
	if err != nil {
		t.Fatalf("hostnameForLocalOverlays: %v", err)
	}
	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeOverlay(t, planDir, "common.yml")
	writeOverlay(t, planDir, "by-host/"+host+".yml")

	got, gotErr := runWithResolveLocalOverlays(t, configPath, "--overlays", "off")
	if gotErr != nil {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	if got != nil {
		t.Fatalf("expected nil overlays with --overlays=off, got %v", got)
	}
}

func TestResolveLocalOverlays_OverlaysInvalidValueErrors(t *testing.T) {
	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, gotErr := runWithResolveLocalOverlays(t, configPath, "--overlays", "bogus")
	if gotErr == nil {
		t.Fatalf("expected error for --overlays=bogus, got nil")
	}
}

func TestResolveLocalOverlays_EnvVarTreatedAsExplicit(t *testing.T) {
	// MOONCAKE_HOST is documented as a source for the hostname (spec-51
	// §Open question 2). Setting it implies the operator named the host
	// intentionally, so a missing by-host file is an error, not a silent
	// miss — same semantics as --host.
	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("MOONCAKE_HOST", "envhost")
	_, gotErr := runWithResolveLocalOverlays(t, configPath)
	if gotErr == nil {
		t.Fatalf("expected error when MOONCAKE_HOST names a host without an overlay file, got nil")
	}
	if !strings.Contains(gotErr.Error(), "envhost") {
		t.Fatalf("error %q does not name the env-supplied host", gotErr)
	}
}

func TestResolveLocalOverlays_FlagOverridesEnvVar(t *testing.T) {
	// Source order: --host > MOONCAKE_HOST > derived. If both are set,
	// the flag wins. Set env to a non-existent host, flag to one that
	// has a file — call should succeed and pick the flag's host.
	planDir := t.TempDir()
	configPath := filepath.Join(planDir, "mooncake.yml")
	if err := os.WriteFile(configPath, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	wantByHost := writeOverlay(t, planDir, "by-host/flaghost.yml")

	t.Setenv("MOONCAKE_HOST", "envhost-not-on-disk")
	got, gotErr := runWithResolveLocalOverlays(t, configPath, "--host", "flaghost")
	if gotErr != nil {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	want := []string{wantByHost}
	if !slicesEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHostnameForLocalOverlays_StripsFirstLabel(t *testing.T) {
	// We can't force os.Hostname() to return a specific value cross-platform,
	// but we can assert the post-condition: the result must contain no '.'.
	h, err := hostnameForLocalOverlays()
	if err != nil {
		t.Fatalf("hostnameForLocalOverlays: %v", err)
	}
	if strings.Contains(h, ".") {
		t.Fatalf("hostname %q still contains a dot — first-label trim failed", h)
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
