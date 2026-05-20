package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.yml"), `name: postgres
description: PostgreSQL module
exports:
  default: components/install.yml
  backup:  components/backup.yml
`)
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if idx.Name != "postgres" {
		t.Errorf("Name = %q, want %q", idx.Name, "postgres")
	}
	if len(idx.Exports) != 2 {
		t.Errorf("Exports len = %d, want 2", len(idx.Exports))
	}
}

func TestLoadIndex_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadIndex(dir)
	if err == nil {
		t.Fatal("expected error for missing index.yml")
	}
	if !strings.Contains(err.Error(), "no index.yml") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestLoadIndex_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.yml"), `exports:
  default: x.yml
`)
	_, err := LoadIndex(dir)
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("LoadIndex error = %v, want missing name", err)
	}
}

func TestLoadIndex_EmptyExports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.yml"), `name: pg
exports: {}
`)
	_, err := LoadIndex(dir)
	if err == nil || !strings.Contains(err.Error(), "exports") {
		t.Errorf("LoadIndex error = %v, want empty exports", err)
	}
}

func TestResolveExport(t *testing.T) {
	dir := t.TempDir()
	// Create exported component file
	if err := os.MkdirAll(filepath.Join(dir, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "components", "install.yml"), `name: install
steps: [{ log: "x" }]
`)
	writeFile(t, filepath.Join(dir, "index.yml"), `name: pg
exports:
  default: components/install.yml
`)
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		export string
	}{
		{"explicit default", "default"},
		{"empty resolves to default", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			abs, err := idx.ResolveExport(dir, tc.export)
			if err != nil {
				t.Fatalf("ResolveExport(%q): %v", tc.export, err)
			}
			want := filepath.Join(dir, "components", "install.yml")
			if abs != want {
				t.Errorf("ResolveExport = %q, want %q", abs, want)
			}
		})
	}
}

func TestResolveExport_UnknownName(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "components", "install.yml"), "name: install\nsteps: []\n")
	writeFile(t, filepath.Join(dir, "index.yml"), `name: postgres
exports:
  default: components/install.yml
`)
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = idx.ResolveExport(dir, "backup")
	if err == nil {
		t.Fatal("expected error for missing export")
	}
	if !strings.Contains(err.Error(), `no export "backup"`) {
		t.Errorf("error = %q", err.Error())
	}
	if !strings.Contains(err.Error(), "default") {
		t.Errorf("error should list available exports, got: %q", err.Error())
	}
}

func TestResolveExport_FileMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.yml"), `name: pg
exports:
  default: components/missing.yml
`)
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = idx.ResolveExport(dir, "default")
	if err == nil {
		t.Fatal("expected error for missing component file")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q", err.Error())
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
