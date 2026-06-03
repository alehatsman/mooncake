package binstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryName(t *testing.T) {
	cases := map[string]struct{ goos, goarch, want string }{
		"linux amd64":   {"linux", "amd64", "mooncake_linux_amd64"},
		"darwin arm64":  {"darwin", "arm64", "mooncake_darwin_arm64"},
		"windows amd64": {"windows", "amd64", "mooncake_windows_amd64.exe"},
		"windows arm64": {"windows", "arm64", "mooncake_windows_arm64.exe"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := BinaryName(c.goos, c.goarch); got != c.want {
				t.Errorf("BinaryName(%s,%s)=%q want %q", c.goos, c.goarch, got, c.want)
			}
		})
	}
}

func TestDirHonorsMooncakeHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MOONCAKE_HOME", tmp)
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "bin"); got != want {
		t.Errorf("Dir()=%q want %q", got, want)
	}
}

func TestPathAndLookup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MOONCAKE_HOME", tmp)

	p, err := Path("windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "bin", "mooncake_windows_amd64.exe"); p != want {
		t.Errorf("Path=%q want %q", p, want)
	}

	// Absent first.
	if _, found, err := Lookup("windows", "amd64"); err != nil || found {
		t.Fatalf("Lookup before create: found=%v err=%v", found, err)
	}
	// Create it, then it's found.
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if gotPath, found, err := Lookup("windows", "amd64"); err != nil || !found || gotPath != p {
		t.Fatalf("Lookup after create: path=%q found=%v err=%v", gotPath, found, err)
	}
}
