package envpath

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestAugmentPure(t *testing.T) {
	sep := string(os.PathListSeparator)
	dirs := []string{"/home/aleh/.local/bin", "/home/aleh/bin", "/home/aleh/go/bin"}

	t.Run("prepends missing dirs", func(t *testing.T) {
		got := augment("/usr/bin"+sep+"/bin", dirs)
		want := strings.Join([]string{
			"/home/aleh/.local/bin", "/home/aleh/bin", "/home/aleh/go/bin", "/usr/bin", "/bin",
		}, sep)
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})

	t.Run("skips dirs already present", func(t *testing.T) {
		in := "/home/aleh/.local/bin" + sep + "/usr/bin"
		got := augment(in, dirs)
		// .local/bin already present → only bin + go/bin prepended.
		want := strings.Join([]string{
			"/home/aleh/bin", "/home/aleh/go/bin", "/home/aleh/.local/bin", "/usr/bin",
		}, sep)
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})

	t.Run("empty PATH", func(t *testing.T) {
		got := augment("", dirs)
		if got != strings.Join(dirs, sep) {
			t.Errorf("got %q want %q", got, strings.Join(dirs, sep))
		}
	})

	t.Run("no dirs is a no-op", func(t *testing.T) {
		if got := augment("/usr/bin", nil); got != "/usr/bin" {
			t.Errorf("nil dirs should not change PATH, got %q", got)
		}
	})

	t.Run("deduplicates within dirs", func(t *testing.T) {
		got := augment("/usr/bin", []string{"/opt/bin", "/opt/bin", ""})
		want := "/opt/bin" + sep + "/usr/bin"
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})
}

func TestAugmentIncludesUserBinDirs(t *testing.T) {
	got := Augment("/usr/bin", "/home/aleh")
	for _, want := range []string{"/home/aleh/.local/bin", "/home/aleh/bin", "/home/aleh/go/bin"} {
		if !contains(got, want) {
			t.Errorf("Augment() = %q, missing %q", got, want)
		}
	}
}

func TestAugmentEmptyHomeKeepsPlatformDirs(t *testing.T) {
	got := Augment("/usr/bin", "")
	if runtime.GOOS != "darwin" {
		if got != "/usr/bin" {
			t.Errorf("empty home on %s should not change PATH, got %q", runtime.GOOS, got)
		}
		return
	}
	// darwin: no user dirs to add, but the brew prefix must still land —
	// the brew-installed-mid-run fix must not depend on a home lookup.
	if !contains(got, "/opt/homebrew/bin") {
		t.Errorf("Augment() = %q, missing /opt/homebrew/bin", got)
	}
}

// The regression this package exists for: on macOS, `brew` installed by an
// earlier step must be resolvable by the next one. The prefix is added
// without stat'ing it, so it is on PATH before brew exists.
func TestSystemBinDirsCoversHomebrewOnDarwin(t *testing.T) {
	dirs := systemBinDirs()
	if runtime.GOOS != "darwin" {
		if len(dirs) != 0 {
			t.Errorf("systemBinDirs() on %s = %v, want none", runtime.GOOS, dirs)
		}
		return
	}
	for _, want := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		var found bool
		for _, d := range dirs {
			if d == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("systemBinDirs() = %v, missing %q", dirs, want)
		}
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	Apply()
	first := os.Getenv("PATH")
	Apply()
	if second := os.Getenv("PATH"); second != first {
		t.Errorf("second Apply() changed PATH:\n first  %q\n second %q", first, second)
	}
}

func contains(path, dir string) bool {
	for _, p := range strings.Split(path, string(os.PathListSeparator)) {
		if p == dir {
			return true
		}
	}
	return false
}
