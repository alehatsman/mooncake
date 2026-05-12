//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"reflect"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"

	"gopkg.in/yaml.v3"
)

func TestHandler_BuildBatchInstallCommand(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		manager string
		pkgs    []string
		extra   []string
		want    []string
	}{
		{
			name:    "apt batch",
			manager: pmApt,
			pkgs:    []string{"a", "b", "c"},
			want:    []string{"apt-get", "install", "-y", "a", "b", "c"},
		},
		{
			name:    "apt batch with extra before names",
			manager: pmApt,
			pkgs:    []string{"vim", "git"},
			extra:   []string{"--no-install-recommends"},
			want:    []string{"apt-get", "install", "-y", "--no-install-recommends", "vim", "git"},
		},
		{
			name:    "dnf batch",
			manager: pmDnf,
			pkgs:    []string{"a", "b"},
			want:    []string{"dnf", "install", "-y", "a", "b"},
		},
		{
			name:    "yum batch",
			manager: pmYum,
			pkgs:    []string{"a", "b"},
			want:    []string{"yum", "install", "-y", "a", "b"},
		},
		{
			name:    "pacman batch with --needed",
			manager: pmPacman,
			pkgs:    []string{"a", "b", "c"},
			want:    []string{"pacman", "-S", "--noconfirm", "--needed", "a", "b", "c"},
		},
		{
			name:    "zypper batch",
			manager: pmZypper,
			pkgs:    []string{"a", "b"},
			want:    []string{"zypper", "install", "-y", "a", "b"},
		},
		{
			name:    "apk batch",
			manager: pmApk,
			pkgs:    []string{"a", "b"},
			want:    []string{"apk", "add", "a", "b"},
		},
		{
			name:    "brew batch",
			manager: pmBrew,
			pkgs:    []string{"a", "b"},
			want:    []string{"brew", "install", "a", "b"},
		},
		{
			name:    "port batch",
			manager: pmPort,
			pkgs:    []string{"a", "b"},
			want:    []string{"port", "install", "a", "b"},
		},
		{
			name:    "choco batch",
			manager: pmChoco,
			pkgs:    []string{"a", "b"},
			want:    []string{"choco", "install", "-y", "a", "b"},
		},
		{
			name:    "scoop batch",
			manager: pmScoop,
			pkgs:    []string{"a", "b"},
			want:    []string{"scoop", "install", "a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.buildBatchInstallCommand(tt.manager, tt.pkgs, false, tt.extra)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildBatchInstallCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_BuildBatchRemoveCommand(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		manager string
		pkgs    []string
		extra   []string
		want    []string
	}{
		{
			name:    "apt batch remove",
			manager: pmApt,
			pkgs:    []string{"a", "b"},
			want:    []string{"apt-get", "remove", "-y", "a", "b"},
		},
		{
			name:    "pacman batch remove",
			manager: pmPacman,
			pkgs:    []string{"a", "b"},
			want:    []string{"pacman", "-R", "--noconfirm", "a", "b"},
		},
		{
			name:    "brew batch uninstall",
			manager: pmBrew,
			pkgs:    []string{"a", "b"},
			want:    []string{"brew", "uninstall", "a", "b"},
		},
		{
			name:    "apk batch del",
			manager: pmApk,
			pkgs:    []string{"a", "b"},
			want:    []string{"apk", "del", "a", "b"},
		},
		{
			name:    "scoop batch uninstall",
			manager: pmScoop,
			pkgs:    []string{"a", "b"},
			want:    []string{"scoop", "uninstall", "a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.buildBatchRemoveCommand(tt.manager, tt.pkgs, tt.extra)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildBatchRemoveCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHandler_BuildInstallCommand_BackwardCompat asserts the single-package
// shim still produces the same output as the batched form with one element.
func TestHandler_BuildInstallCommand_BackwardCompat(t *testing.T) {
	h := &Handler{}
	got := h.buildInstallCommand(pmApt, "vim", false, []string{"--no-install-recommends"})
	want := []string{"apt-get", "install", "-y", "--no-install-recommends", "vim"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildInstallCommand single-package = %v, want %v", got, want)
	}
}

// TestPackage_UnmarshalYAML_ListNames asserts the existing list form still
// populates Names directly.
func TestPackage_UnmarshalYAML_ListNames(t *testing.T) {
	yamlSrc := `
name: ""
names: [vim, git, curl]
state: present
manager: pacman
`
	var p config.Package
	if err := yaml.Unmarshal([]byte(yamlSrc), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(p.Names, []string{"vim", "git", "curl"}) {
		t.Errorf("Names = %v, want [vim git curl]", p.Names)
	}
	if p.NamesExpr != "" {
		t.Errorf("NamesExpr = %q, want empty", p.NamesExpr)
	}
}

// TestPackage_UnmarshalYAML_StringNames asserts that a scalar names field is
// captured into NamesExpr for late template resolution.
func TestPackage_UnmarshalYAML_StringNames(t *testing.T) {
	yamlSrc := `
names: "{{ pacman_packages }}"
state: present
manager: pacman
`
	var p config.Package
	if err := yaml.Unmarshal([]byte(yamlSrc), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Names) != 0 {
		t.Errorf("Names = %v, want empty", p.Names)
	}
	if p.NamesExpr != "{{ pacman_packages }}" {
		t.Errorf("NamesExpr = %q, want %q", p.NamesExpr, "{{ pacman_packages }}")
	}
}

// TestHandler_Validate_NamesExpr asserts the new NamesExpr field satisfies
// Validate's "must have name/names/upgrade" check.
func TestHandler_Validate_NamesExpr(t *testing.T) {
	h := &Handler{}
	step := &config.Step{
		Package: &config.Package{
			NamesExpr: "{{ pkgs }}",
			State:     "present",
		},
	}
	if err := h.Validate(step); err != nil {
		t.Errorf("Validate() with NamesExpr set unexpectedly errored: %v", err)
	}
}

// TestHandler_ResolveNamesExpr exercises the variable-typing paths.
func TestHandler_ResolveNamesExpr(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	ctx.Variables = map[string]interface{}{
		"pkgs_slice":  []string{"a", "b", "c"},
		"pkgs_iface":  []interface{}{"x", "y"},
		"pkgs_scalar": "alpha beta gamma",
	}

	tests := []struct {
		name string
		expr string
		want []string
	}{
		{"slice typed var via {{ }}", "{{ pkgs_slice }}", []string{"a", "b", "c"}},
		{"interface slice var via {{ }}", "{{ pkgs_iface }}", []string{"x", "y"}},
		{"raw whitespace string after render", "alpha beta gamma", []string{"alpha", "beta", "gamma"}},
		{"raw pongo-stringified list", "[a b c]", []string{"a", "b", "c"}},
		{"raw json list", `["a","b","c"]`, []string{"a", "b", "c"}},
		{"raw yaml flow list", "[a, b, c]", []string{"a", "b", "c"}},
		{"comma-separated scalar", "a, b, c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.resolveNamesExpr(ctx, tt.expr)
			if err != nil {
				t.Fatalf("resolveNamesExpr(%q) error: %v", tt.expr, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveNamesExpr(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

// TestHandler_Execute_TemplatedNames asserts Execute resolves NamesExpr and
// proceeds with the populated package list. Since no real package manager
// runs in the test environment, we just verify expansion runs without panic
// (the actual install will fail at exec, which is fine).
func TestHandler_Execute_TemplatedNames(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	ctx.Variables = map[string]interface{}{
		"pkgs": []string{"a", "b"},
	}
	step := &config.Step{
		Package: &config.Package{
			NamesExpr: "{{ pkgs }}",
			State:     "present",
			Manager:   pmApt,
		},
	}

	// The handler will attempt to check installation status (which exits
	// non-zero for synthetic packages) and then try to install — both fine
	// for testing the resolution path. We only assert that resolution
	// itself did not produce an error.
	_, err := h.Execute(ctx, step)
	if err != nil {
		// The error string must not be about resolving names — that's the
		// failure mode this test is guarding against.
		if errStr := err.Error(); containsString(errStr, "resolve package names expression") {
			t.Errorf("Execute() failed to resolve NamesExpr: %v", err)
		}
	}
}

func containsString(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
