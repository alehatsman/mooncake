package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/binstore"
)

func TestResolveBinary_ExplicitWins(t *testing.T) {
	// Even with an empty store, an explicit path is returned verbatim
	// (platform validation happens later, in VerifyBinaryPlatform).
	t.Setenv("MOONCAKE_HOME", t.TempDir())
	got, err := ResolveBinary("windows", "arm64", "/some/explicit/mooncake.exe")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/some/explicit/mooncake.exe" {
		t.Errorf("explicit not honored: %q", got)
	}
}

func TestResolveBinary_FromStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MOONCAKE_HOME", home)
	// Seed a store entry for a target distinct from the host so the
	// self-fallback can't be what satisfies the lookup.
	p, err := binstore.Path("windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveBinary("windows", "amd64", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Errorf("ResolveBinary=%q want store path %q", got, p)
	}
}

func TestResolveBinary_SelfFallbackWhenMatch(t *testing.T) {
	// Empty store, target == host platform → the controller's own binary
	// (the test binary) is a valid match.
	t.Setenv("MOONCAKE_HOME", t.TempDir())
	got, err := ResolveBinary(runtime.GOOS, runtime.GOARCH, "")
	if err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	if got != self {
		t.Errorf("ResolveBinary=%q want self %q", got, self)
	}
}

func TestResolveBinary_ErrorPointsAtSelfbuild(t *testing.T) {
	// Empty store, target a platform the host binary can't be → error.
	t.Setenv("MOONCAKE_HOME", t.TempDir())
	wantOS := "windows"
	if runtime.GOOS == "windows" {
		wantOS = "linux"
	}
	_, err := ResolveBinary(wantOS, runtime.GOARCH, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "selfbuild") {
		t.Errorf("error should point at selfbuild: %v", err)
	}
}
