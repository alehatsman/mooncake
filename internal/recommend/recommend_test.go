package recommend

import (
	"slices"
	"testing"

	"github.com/alehatsman/mooncake/internal/facts"
)

func TestProfileFrom_NilFacts(t *testing.T) {
	p := ProfileFrom(nil)
	if p.OS != "" || p.PackageManager != "" {
		t.Errorf("nil facts should yield zero Profile, got %+v", p)
	}
}

func TestProfileFrom_PopulatedFacts(t *testing.T) {
	f := &facts.Facts{OS: "linux", PackageManager: "pacman"}
	p := ProfileFrom(f)
	if p.OS != "linux" || p.PackageManager != "pacman" {
		t.Errorf("unexpected Profile: %+v", p)
	}
}

func TestRecommend_BaseFirst(t *testing.T) {
	got := Recommend(Profile{OS: "linux", PackageManager: "pacman"}, nil, 0)
	if len(got) == 0 {
		t.Fatal("expected non-empty recommendations")
	}
	// First entry must be a base entry — base ordering is the contract.
	first := got[0]
	wantFirstSet := []string{"git", "zsh", "tmux", "neovim", "fzf", "ripgrep"}
	if !slices.Contains(wantFirstSet, first) {
		t.Errorf("first entry %q is not in the base set %v", first, wantFirstSet)
	}
}

func TestRecommend_Deduplication(t *testing.T) {
	// docker appears in apt + pacman + dnf. Only one should land in any
	// given output.
	got := Recommend(Profile{OS: "linux", PackageManager: "apt"}, nil, 0)
	count := 0
	for _, name := range got {
		if name == "docker" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("docker appeared %d times; expected at most 1", count)
	}
}

func TestRecommend_ProfileSpecific(t *testing.T) {
	pacman := Recommend(Profile{OS: "linux", PackageManager: "pacman"}, nil, 0)
	apt := Recommend(Profile{OS: "linux", PackageManager: "apt"}, nil, 0)

	if !slices.Contains(pacman, "base-devel") {
		t.Error("pacman profile should include base-devel")
	}
	if slices.Contains(pacman, "build-essential") {
		t.Error("pacman profile must NOT include build-essential (apt-specific)")
	}
	if !slices.Contains(apt, "build-essential") {
		t.Error("apt profile should include build-essential")
	}
}

func TestRecommend_LimitCaps(t *testing.T) {
	got := Recommend(Profile{OS: "linux", PackageManager: "apt"}, nil, 3)
	if len(got) != 3 {
		t.Errorf("limit=3 should cap to 3, got %d (%v)", len(got), got)
	}
}

func TestRecommend_LimitZeroReturnsAll(t *testing.T) {
	zero := Recommend(Profile{OS: "linux", PackageManager: "apt"}, nil, 0)
	big := Recommend(Profile{OS: "linux", PackageManager: "apt"}, nil, 999)
	if !slices.Equal(zero, big) {
		t.Errorf("limit=0 and limit=999 should match:\n  0: %v\n  999: %v", zero, big)
	}
}

func TestRecommend_FilterByKnown(t *testing.T) {
	// Pretend only zsh is installed locally.
	known := map[string]bool{"zsh": true}
	got := Recommend(Profile{OS: "linux", PackageManager: "pacman"}, known, 0)
	if len(got) != 1 || got[0] != "zsh" {
		t.Errorf("filter to known={zsh}: got %v", got)
	}
}

func TestRecommend_UnknownProfileGetsBaseOnly(t *testing.T) {
	got := Recommend(Profile{OS: "haiku", PackageManager: "pkgsrc"}, nil, 0)
	for _, name := range got {
		// All base entries are listed first; verify none are
		// linux-specific (build-essential, base-devel, etc.).
		linuxOnly := []string{"build-essential", "base-devel", "ufw", "docker", "homebrew", "iterm2", "sshd-hardening"}
		if slices.Contains(linuxOnly, name) {
			t.Errorf("unknown profile leaked profile-specific entry %q", name)
		}
	}
}

func TestRecommend_DarwinDistinct(t *testing.T) {
	darwin := Recommend(Profile{OS: "darwin"}, nil, 0)
	if !slices.Contains(darwin, "homebrew") {
		t.Error("darwin profile should include homebrew")
	}
	if slices.Contains(darwin, "build-essential") {
		t.Error("darwin profile must NOT include build-essential (linux+apt)")
	}
}
