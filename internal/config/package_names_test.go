package config

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestPackageNames_ScalarRoundTrip asserts the YAML reader accepts both list
// and scalar forms of `names:` and routes each to the right struct field.
func TestPackageNames_ScalarRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantList []string
		wantExpr string
	}{
		{
			name: "list of names",
			yaml: `
- name: install
  package:
    manager: pacman
    names: [vim, git, curl]
    state: present
`,
			wantList: []string{"vim", "git", "curl"},
			wantExpr: "",
		},
		{
			name: "templated names",
			yaml: `
- name: install
  package:
    manager: pacman
    names: "{{ pacman_packages }}"
    state: present
`,
			wantList: nil,
			wantExpr: "{{ pacman_packages }}",
		},
	}

	reader := NewYAMLConfigReader()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp(t.TempDir(), "*.yml")
			if err != nil {
				t.Fatalf("temp: %v", err)
			}
			if _, err := tmpFile.WriteString(tt.yaml); err != nil {
				t.Fatalf("write: %v", err)
			}
			tmpFile.Close()

			parsed, err := reader.ReadConfig(tmpFile.Name())
			if err != nil {
				t.Fatalf("ReadConfig: %v", err)
			}
			if len(parsed.Steps) != 1 {
				t.Fatalf("got %d steps, want 1", len(parsed.Steps))
			}
			pkg := parsed.Steps[0].Pkg
			if pkg == nil {
				t.Fatal("Package is nil")
			}
			if pkg.NamesExpr != tt.wantExpr {
				t.Errorf("NamesExpr = %q, want %q", pkg.NamesExpr, tt.wantExpr)
			}
			if len(pkg.Names) != len(tt.wantList) {
				t.Fatalf("Names = %v, want %v", pkg.Names, tt.wantList)
			}
			for i, n := range pkg.Names {
				if n != tt.wantList[i] {
					t.Errorf("Names[%d] = %q, want %q", i, n, tt.wantList[i])
				}
			}
		})
	}
}

// TestPackage_UnmarshalYAML_RejectsNonStringItems asserts the unmarshaller
// produces a clear error when the list contains non-string entries.
func TestPackage_UnmarshalYAML_RejectsNonStringItems(t *testing.T) {
	src := `
names:
  - vim
  - 42
state: present
`
	var p Package
	err := yaml.Unmarshal([]byte(src), &p)
	if err == nil {
		t.Fatal("expected error for non-string item in names")
	}
}
