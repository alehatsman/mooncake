package config

import "testing"

func TestComponentRefKindOf(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want ComponentRefKind
	}{
		{"empty", "", ComponentRefPreset},
		{"bare preset", "ollama", ComponentRefPreset},
		{"alias single", "postgres", ComponentRefPreset}, // syntactic; alias-vs-preset decided at runtime
		{"alias with export", "postgres/backup", ComponentRefAlias},
		{"relative path", "./components/foo.yml", ComponentRefLocalPath},
		{"parent relative", "../shared/foo.yml", ComponentRefLocalPath},
		{"absolute path", "/etc/components/foo.yml", ComponentRefLocalPath},
		{"remote", "github.com/owner/repo@v1.0.0", ComponentRefRemote},
		{"remote with subpath", "github.com/owner/repo/sub@v1.0.0", ComponentRefRemote},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComponentRefKindOf(tc.ref); got != tc.want {
				t.Errorf("ComponentRefKindOf(%q) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestSplitComponentAlias(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantAlias  string
		wantExport string
	}{
		{"bare", "postgres", "postgres", "default"},
		{"with export", "postgres/backup", "postgres", "backup"},
		{"empty", "", "", ""},
		{"remote skips", "github.com/o/r@v1", "", ""},
		{"local skips", "./foo.yml", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alias, export := SplitComponentAlias(tc.ref)
			if alias != tc.wantAlias || export != tc.wantExport {
				t.Errorf("SplitComponentAlias(%q) = (%q, %q), want (%q, %q)",
					tc.ref, alias, export, tc.wantAlias, tc.wantExport)
			}
		})
	}
}
