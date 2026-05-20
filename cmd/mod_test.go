package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestUpsertModulesEntry_NewFile creates a fresh playbook with only a modules: block.
func TestUpsertModulesEntry_NewFile(t *testing.T) {
	dir := t.TempDir()
	playbook := filepath.Join(dir, "mooncake.yml")
	if err := upsertModulesEntry(playbook, "postgres", "github.com/mooncake-modules/postgres@v1.0.0"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got := readMap(t, playbook)
	mods, ok := got["modules"].(map[string]interface{})
	if !ok {
		t.Fatalf("modules: not a map: %T", got["modules"])
	}
	if mods["postgres"] != "github.com/mooncake-modules/postgres@v1.0.0" {
		t.Errorf("modules.postgres = %v", mods["postgres"])
	}
}

// TestUpsertModulesEntry_ExistingPlaybook merges into an existing modules: map
// without nuking other top-level keys.
func TestUpsertModulesEntry_ExistingPlaybook(t *testing.T) {
	dir := t.TempDir()
	playbook := filepath.Join(dir, "mooncake.yml")
	initial := `version: "1.0"
modules:
  redis: github.com/mooncake-modules/redis@v0.5.0
steps:
  - name: noop
    log: "x"
`
	if err := os.WriteFile(playbook, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := upsertModulesEntry(playbook, "postgres", "github.com/mooncake-modules/postgres@v1.0.0"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got := readMap(t, playbook)
	if got["version"] != "1.0" {
		t.Errorf("version field lost: %v", got["version"])
	}
	mods := got["modules"].(map[string]interface{})
	if mods["postgres"] != "github.com/mooncake-modules/postgres@v1.0.0" {
		t.Errorf("postgres not added")
	}
	if mods["redis"] != "github.com/mooncake-modules/redis@v0.5.0" {
		t.Errorf("redis lost: %v", mods["redis"])
	}
	if got["steps"] == nil {
		t.Error("steps lost")
	}
}

// TestUpsertModulesEntry_Overwrite re-adding the same alias updates the
// version rather than duplicating.
func TestUpsertModulesEntry_Overwrite(t *testing.T) {
	dir := t.TempDir()
	playbook := filepath.Join(dir, "mooncake.yml")
	if err := upsertModulesEntry(playbook, "pg", "github.com/x/pg@v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := upsertModulesEntry(playbook, "pg", "github.com/x/pg@v2.0.0"); err != nil {
		t.Fatal(err)
	}
	got := readMap(t, playbook)
	mods := got["modules"].(map[string]interface{})
	if mods["pg"] != "github.com/x/pg@v2.0.0" {
		t.Errorf("alias not updated: %v", mods["pg"])
	}
	if len(mods) != 1 {
		t.Errorf("expected 1 entry, got %d", len(mods))
	}
}

func TestWalkCache(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"github.com/owner-a/repo1@v1.0.0",
		"github.com/owner-a/repo2@v0.1.0",
		"gitlab.com/team/proj@v3.0.0",
	} {
		if err := os.MkdirAll(filepath.Join(root, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := walkCache(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"github.com/owner-a/repo1@v1.0.0",
		"github.com/owner-a/repo2@v0.1.0",
		"gitlab.com/team/proj@v3.0.0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("walkCache:\n got:  %v\n want: %v", got, want)
	}
}

func readMap(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
