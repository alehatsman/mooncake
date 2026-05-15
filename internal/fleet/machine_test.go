package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMachineFile is a small helper to seed test machine layouts.
func writeMachineFile(t *testing.T, root, machine, name, content string) string {
	t.Helper()
	dir := filepath.Join(root, "machines", machine)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestLookupMachineManifest_MissingFileIsNotFound(t *testing.T) {
	root := t.TempDir()
	_, found, err := LookupMachineManifest(root, "no-such")
	if err != nil {
		t.Fatalf("expected nil error on missing manifest, got %v", err)
	}
	if found {
		t.Fatalf("expected found=false for missing manifest")
	}
}

func TestLookupMachineManifest_FindsExistingFile(t *testing.T) {
	root := t.TempDir()
	want := writeMachineFile(t, root, "main_pc", MachineManifestName,
		"phases:\n  - {name: a, peer: x, plan: ./index.yml}\n")

	got, found, err := LookupMachineManifest(root, "main_pc")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestLookupMachineManifest_DirectoryAtManifestPathErrors(t *testing.T) {
	// Defensive: someone with `mkdir machines/foo/fleet.yml/` (typo from a
	// build script, etc.) must get a clear error, not a silent fall-through
	// to the index.yml branch.
	root := t.TempDir()
	dir := filepath.Join(root, "machines", "foo", MachineManifestName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, _, err := LookupMachineManifest(root, "foo")
	if err == nil {
		t.Fatalf("expected error when manifest path is a directory")
	}
}

func TestLoadMachineManifest_ValidTwoPhase(t *testing.T) {
	// The canonical Windows+WSL machine layout: phase 1 boots the Windows
	// host plan, phase 2 boots the WSL guest plan. Phase ordering must be
	// preserved exactly.
	root := t.TempDir()
	path := writeMachineFile(t, root, "main_pc", MachineManifestName, `
phases:
  - name: windows-host
    peer: main_pc-win
    plan: ../../platforms/windows/bootstrap.yml
    vars:
      - ./windows.yml
    tags: [windows]
  - name: wsl
    peer: main_pc
    plan: ./index.yml
`)

	m, err := LoadMachineManifest(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Phases) != 2 {
		t.Fatalf("phases = %d, want 2", len(m.Phases))
	}
	if m.Phases[0].Name != "windows-host" || m.Phases[1].Name != "wsl" {
		t.Errorf("phase ordering lost: %+v", m.Phases)
	}
	// Paths must be resolved to absolute and rooted at the manifest dir.
	manifestDir := filepath.Dir(path)
	wantPlan0 := filepath.Clean(filepath.Join(manifestDir, "../../platforms/windows/bootstrap.yml"))
	if m.Phases[0].Plan != wantPlan0 {
		t.Errorf("phase 0 plan = %q, want %q", m.Phases[0].Plan, wantPlan0)
	}
	wantVars0 := filepath.Clean(filepath.Join(manifestDir, "./windows.yml"))
	if len(m.Phases[0].Vars) != 1 || m.Phases[0].Vars[0] != wantVars0 {
		t.Errorf("phase 0 vars = %v, want [%s]", m.Phases[0].Vars, wantVars0)
	}
	if len(m.Phases[0].Tags) != 1 || m.Phases[0].Tags[0] != "windows" {
		t.Errorf("phase 0 tags lost: %v", m.Phases[0].Tags)
	}
}

func TestLoadMachineManifest_AbsolutePlanPreserved(t *testing.T) {
	// An operator may point a phase at a plan outside the conventional
	// layout (an absolute path). Resolution must pass it through
	// unchanged instead of re-rooting it under the manifest dir.
	root := t.TempDir()
	absPlan := filepath.Join(root, "other.yml") // arbitrary absolute path
	path := writeMachineFile(t, root, "m", MachineManifestName,
		"phases:\n  - {name: p, peer: x, plan: "+absPlan+"}\n")

	m, err := LoadMachineManifest(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Phases[0].Plan != filepath.Clean(absPlan) {
		t.Errorf("absolute plan rewritten: got %q want %q", m.Phases[0].Plan, absPlan)
	}
}

func TestMachineManifest_ValidateRejects(t *testing.T) {
	tests := []struct {
		name    string
		m       MachineManifest
		wantErr string
	}{
		{"empty phases", MachineManifest{}, "phases list is empty"},
		{
			"missing name",
			MachineManifest{Phases: []MachinePhase{{Peer: "p", Plan: "x"}}},
			"name is empty",
		},
		{
			"missing peer",
			MachineManifest{Phases: []MachinePhase{{Name: "p", Plan: "x"}}},
			"peer is empty",
		},
		{
			"missing plan",
			MachineManifest{Phases: []MachinePhase{{Name: "p", Peer: "x"}}},
			"plan is empty",
		},
		{
			"duplicate name",
			MachineManifest{Phases: []MachinePhase{
				{Name: "p", Peer: "x", Plan: "y"},
				{Name: "p", Peer: "x2", Plan: "y2"},
			}},
			`duplicate phase name "p"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadMachineManifest_MalformedYAMLErrors(t *testing.T) {
	root := t.TempDir()
	path := writeMachineFile(t, root, "m", MachineManifestName, "phases: [\n  - not yaml")
	_, err := LoadMachineManifest(path)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
