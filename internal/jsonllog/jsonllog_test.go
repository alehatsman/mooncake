package jsonllog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppend_CreatesDirAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "log.jsonl")

	type entry struct {
		ID string `json:"id"`
	}
	if err := Append(path, entry{ID: "first"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Directory was created with the expected perm bits.
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir perms = %o, want 0700", info.Mode().Perm())
	}

	// File was created with the expected perm bits.
	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file perms = %o, want 0600", info.Mode().Perm())
	}
}

func TestAppend_OneLinePerCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	type entry struct {
		ID string `json:"id"`
	}
	for _, id := range []string{"a", "b", "c"} {
		if err := Append(path, entry{ID: id}); err != nil {
			t.Fatalf("Append %q: %v", id, err)
		}
	}

	body, err := os.ReadFile(path) // #nosec G304 -- path under t.TempDir
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 — body=%q", len(lines), string(body))
	}
	for i, line := range lines {
		var got entry
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("line %d not valid JSON: %v (%q)", i, err, line)
		}
	}
}

func TestAppend_PreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	// Seed with a pre-existing line.
	if err := os.WriteFile(path, []byte(`{"id":"seed"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Append(path, map[string]string{"id": "added"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	body, _ := os.ReadFile(path) // #nosec G304 -- path under t.TempDir
	got := strings.TrimRight(string(body), "\n")
	want := `{"id":"seed"}` + "\n" + `{"id":"added"}`
	if got != want {
		t.Errorf("body mismatch:\n got:  %q\n want: %q", got, want)
	}
}
