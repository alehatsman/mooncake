//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"encoding/json"
	"testing"
)

// Trimmed from a real `brew info --json=v2 --installed` payload, keeping one
// package per aliasing class that used to be reported as missing forever.
const brewInfoFixture = `{
  "formulae": [
    {"name": "packer",      "full_name": "hashicorp/tap/packer", "aliases": [],                                "oldnames": []},
    {"name": "python@3.14", "full_name": "python@3.14",          "aliases": ["python", "python3", "python@3"], "oldnames": []},
    {"name": "jq",          "full_name": "jq",                   "aliases": [],                                "oldnames": []},
    {"name": "openjdk",     "full_name": "openjdk",              "aliases": [],                                "oldnames": ["java"]}
  ],
  "casks": [
    {"token": "docker-desktop", "full_token": "docker-desktop", "old_tokens": ["docker"]},
    {"token": "vlc",            "full_token": "vlc",            "old_tokens": []}
  ]
}`

func parseFixture(t *testing.T) *brewInfoInstalled {
	t.Helper()
	var info brewInfoInstalled
	if err := json.Unmarshal([]byte(brewInfoFixture), &info); err != nil {
		t.Fatalf("fixture does not parse: %v", err)
	}
	return &info
}

// TestBrewNameSet_Formulae covers the three ways a playbook name legitimately
// differs from the one `brew list` prints. Each miss here is a step that
// reinstalls on every apply and never converges.
func TestBrewNameSet_Formulae(t *testing.T) {
	set := brewNameSet(parseFixture(t), false)

	installed := []string{
		"packer",               // canonical
		"hashicorp/tap/packer", // tap-qualified, as playbooks spell it
		"python@3.14",          // canonical versioned formula
		"python",               // alias for it
		"python3",              // ditto
		"jq",
		"openjdk",
		"java", // old name
	}
	for _, name := range installed {
		if _, ok := set[name]; !ok {
			t.Errorf("%q should count as installed", name)
		}
	}

	for _, name := range []string{"ripgrep", "python@3.9", "docker-desktop", "vlc"} {
		if _, ok := set[name]; ok {
			t.Errorf("%q must not count as installed (casks and absent formulae are a different set)", name)
		}
	}
}

// TestBrewNameSet_Casks covers the cask rename that made `docker` reinstall on
// every apply after Homebrew renamed the cask to docker-desktop.
func TestBrewNameSet_Casks(t *testing.T) {
	set := brewNameSet(parseFixture(t), true)

	for _, name := range []string{"docker-desktop", "docker", "vlc"} {
		if _, ok := set[name]; !ok {
			t.Errorf("%q should count as an installed cask", name)
		}
	}
	for _, name := range []string{"jq", "packer", "slack"} {
		if _, ok := set[name]; ok {
			t.Errorf("%q must not count as an installed cask", name)
		}
	}
}

// TestBrewNameSet_Empty is the fresh-machine case: nothing installed, so
// nothing matches and every package is queued for install.
func TestBrewNameSet_Empty(t *testing.T) {
	var info brewInfoInstalled
	if err := json.Unmarshal([]byte(`{"formulae":[],"casks":[]}`), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if n := len(brewNameSet(&info, false)); n != 0 {
		t.Errorf("formula set = %d entries, want 0", n)
	}
	if n := len(brewNameSet(&info, true)); n != 0 {
		t.Errorf("cask set = %d entries, want 0", n)
	}
}
