package tool

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// fakeMiseRunner records invocations and returns canned responses,
// without shelling out to a real mise binary.
type fakeMiseRunner struct {
	lookPathErr error

	installs     []string // "tool@version"
	installErrs  map[string]error
	installCalls int

	// whichResponses keyed by "tool@version" → bin path. Missing key →
	// returns an error (which Locate interprets as "not installed").
	whichResponses map[string]string
	whichCalls     int
}

func (f *fakeMiseRunner) lookPath() error { return f.lookPathErr }

func (f *fakeMiseRunner) install(_ context.Context, tool, version string, _ map[string]string) error {
	key := tool + "@" + version
	f.installs = append(f.installs, key)
	f.installCalls++
	if err, ok := f.installErrs[key]; ok {
		return err
	}
	// Pretend the install populates `which` afterwards so an immediate
	// Locate succeeds. Default path: /opt/mise/<tool>/<version>/bin/<tool>.
	if f.whichResponses == nil {
		f.whichResponses = map[string]string{}
	}
	f.whichResponses[key] = "/opt/mise/" + tool + "/" + version + "/bin/" + tool
	return nil
}

func (f *fakeMiseRunner) which(_ context.Context, tool, version string) (string, error) {
	f.whichCalls++
	if path, ok := f.whichResponses[tool+"@"+version]; ok {
		return path, nil
	}
	return "", errors.New("not installed")
}

func withFakeMise(t *testing.T, f *fakeMiseRunner) {
	t.Helper()
	prev := defaultMiseRunner
	defaultMiseRunner = f
	t.Cleanup(func() { defaultMiseRunner = prev })
}

func TestMiseValidate(t *testing.T) {
	withFakeMise(t, &fakeMiseRunner{}) // mise on PATH

	b, err := Get(BackendMise)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		tool    *config.Tool
		wantErr bool
	}{
		{"ok minimal", &config.Tool{}, false},
		{"ok with mise_tool", &config.Tool{MiseTool: "node"}, false},
		{"reject url", &config.Tool{URL: "https://x"}, true},
		{"reject repo", &config.Tool{Repo: "owner/name"}, true},
		{"reject asset", &config.Tool{Asset: "x.zip"}, true},
		{"reject tag", &config.Tool{Tag: "v1"}, true},
		{"reject checksum", &config.Tool{Checksum: "sha256:abc"}, true},
		{"reject strip_components", &config.Tool{StripComponents: 1}, true},
		{"reject bin", &config.Tool{Bin: "bin/x"}, true},
		{"reject write_tool_versions", &config.Tool{WriteToolVersions: true}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := b.Validate(tc.tool)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestMiseValidateRequiresBinaryOnPath(t *testing.T) {
	withFakeMise(t, &fakeMiseRunner{lookPathErr: errors.New("not found")})

	b, _ := Get(BackendMise)
	err := b.Validate(&config.Tool{})
	if err == nil {
		t.Fatal("expected validation error when mise is missing from PATH")
	}
}

func TestMisePlanIsBackendOwned(t *testing.T) {
	withFakeMise(t, &fakeMiseRunner{})

	b, _ := Get(BackendMise)
	plan, err := b.Plan(context.Background(), Spec{Name: "node", Version: "24.0.0"}, FactSnapshot{})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.UseSharedPipeline {
		t.Error("mise Plan must set UseSharedPipeline=false")
	}
}

func TestMiseInstallShellsOutWithDefaultName(t *testing.T) {
	f := &fakeMiseRunner{}
	withFakeMise(t, f)

	b, _ := Get(BackendMise)
	err := b.Install(context.Background(), Spec{Name: "node", Version: "24.0.0"}, Plan{}, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := f.installs; len(got) != 1 || got[0] != "node@24.0.0" {
		t.Errorf("installs = %v, want [node@24.0.0]", got)
	}
}

func TestMiseInstallUsesMiseToolOverride(t *testing.T) {
	f := &fakeMiseRunner{}
	withFakeMise(t, f)

	b, _ := Get(BackendMise)
	err := b.Install(context.Background(), Spec{
		Name:     "java",
		Version:  "21.0.5",
		MiseTool: "temurin",
	}, Plan{}, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := f.installs; len(got) != 1 || got[0] != "temurin@21.0.5" {
		t.Errorf("installs = %v, want [temurin@21.0.5]", got)
	}
}

func TestMiseLocateMissingReturnsEmpty(t *testing.T) {
	withFakeMise(t, &fakeMiseRunner{})

	b, _ := Get(BackendMise)
	bin, err := b.Locate(context.Background(), Spec{Name: "node", Version: "24.0.0"}, "")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if bin != "" {
		t.Errorf("expected empty bin for missing tool, got %q", bin)
	}
}

func TestMiseLocateReturnsBinPath(t *testing.T) {
	f := &fakeMiseRunner{
		whichResponses: map[string]string{"node@24.0.0": "/opt/mise/node/24.0.0/bin/node"},
	}
	withFakeMise(t, f)

	b, _ := Get(BackendMise)
	bin, err := b.Locate(context.Background(), Spec{Name: "node", Version: "24.0.0"}, "")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if bin != "/opt/mise/node/24.0.0/bin/node" {
		t.Errorf("bin = %q", bin)
	}
	if filepath.Dir(bin) != "/opt/mise/node/24.0.0/bin" {
		t.Errorf("dir = %q", filepath.Dir(bin))
	}
}

// withPATH replaces PATH for the duration of the test. Used to verify
// the real runner's fallback when `mise` is not on PATH.
func withPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

func TestRealMiseRunner_FallsBackToMooncakeStore(t *testing.T) {
	// Isolate the mooncake store under XDG_DATA_HOME.
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	storeRoot := filepath.Join(xdg, "mooncake", "tools")

	// PATH points only at an empty dir → exec.LookPath("mise") will miss.
	emptyDir := t.TempDir()
	withPATH(t, emptyDir)

	// Seed a fake mise binary in the mooncake store.
	miseBin := filepath.Join(storeRoot, "mise", "2026.5.6", "bin", "mise")
	if err := os.MkdirAll(filepath.Dir(miseBin), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(miseBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake mise: %v", err)
	}

	r := realMiseRunner{}
	if err := r.lookPath(); err != nil {
		t.Fatalf("lookPath should succeed via mooncake-store fallback: %v", err)
	}
	resolved, err := r.resolveMisePath()
	if err != nil {
		t.Fatalf("resolveMisePath: %v", err)
	}
	if resolved != miseBin {
		t.Errorf("resolved = %q, want %q", resolved, miseBin)
	}
}

func TestRealMiseRunner_NoMiseAnywhereFails(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	withPATH(t, t.TempDir())

	r := realMiseRunner{}
	if err := r.lookPath(); err == nil {
		t.Fatal("lookPath should fail when mise is nowhere")
	}
}

func TestFindMooncakeManagedMise_MissingStoreReturnsEmpty(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg) // no mooncake dir yet

	if got := findMooncakeManagedMise(); got != "" {
		t.Errorf("expected empty when store missing, got %q", got)
	}
}

func TestMergeEnvOverlays(t *testing.T) {
	base := []string{"PATH=/usr/bin", "FOO=old"}
	got := mergeEnv(base, map[string]string{"FOO": "new", "BAR": "x"})
	hasFooNew := false
	hasFooOld := false
	hasBar := false
	for _, e := range got {
		switch e {
		case "FOO=new":
			hasFooNew = true
		case "FOO=old":
			hasFooOld = true
		case "BAR=x":
			hasBar = true
		}
	}
	if !hasFooNew || !hasBar {
		t.Errorf("expected FOO=new and BAR=x in env: %v", got)
	}
	if hasFooOld {
		t.Errorf("expected FOO=old to be overridden: %v", got)
	}
}
