package fleet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureControllerID_GeneratesOnFirstCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "controller_id")
	id, err := ensureControllerIDAt(path)
	if err != nil {
		t.Fatalf("ensureControllerIDAt: %v", err)
	}
	if !isValidUUIDv4(id) {
		t.Errorf("generated id %q is not a valid UUIDv4", id)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 0600", got)
	}
}

func TestEnsureControllerID_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller_id")
	first, err := ensureControllerIDAt(path)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := ensureControllerIDAt(path)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Errorf("id changed between calls: %q vs %q", first, second)
	}
}

func TestEnsureControllerID_RegeneratesMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller_id")
	// Seed with garbage that won't pass isValidUUIDv4.
	if err := os.WriteFile(path, []byte("not-a-uuid\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	id, err := ensureControllerIDAt(path)
	if err != nil {
		t.Fatalf("ensureControllerIDAt: %v", err)
	}
	if !isValidUUIDv4(id) {
		t.Errorf("expected regenerated valid UUIDv4, got %q", id)
	}
	// Subsequent call returns the same regenerated id.
	again, err := ensureControllerIDAt(path)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if again != id {
		t.Errorf("id not stable after regen: %q vs %q", id, again)
	}
}

func TestEnsureControllerID_RejectsEmptyPath(t *testing.T) {
	if _, err := ensureControllerIDAt(""); err == nil {
		t.Fatal("want error on empty path")
	}
}

func TestEnsureControllerID_TrimsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller_id")
	valid, _ := newUUIDv4()
	if err := os.WriteFile(path, []byte("  "+valid+"  \n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := ensureControllerIDAt(path)
	if err != nil {
		t.Fatalf("ensureControllerIDAt: %v", err)
	}
	if got != valid {
		t.Errorf("trimmed id = %q, want %q", got, valid)
	}
}

func TestNewUUIDv4_GeneratesValidShape(t *testing.T) {
	// Sanity: 1000 IDs all pass the shape check and are unique.
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id, err := newUUIDv4()
		if err != nil {
			t.Fatalf("newUUIDv4: %v", err)
		}
		if !isValidUUIDv4(id) {
			t.Errorf("id %q failed shape check", id)
		}
		if _, dup := seen[id]; dup {
			t.Errorf("collision after %d generations: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestIsValidUUIDv4(t *testing.T) {
	cases := map[string]bool{
		// Generated examples, version=4, variant in {8,9,a,b}:
		"00000000-0000-4000-8000-000000000000": true,
		"ffffffff-ffff-4fff-bfff-ffffffffffff": true,
		"deadbeef-1234-4abc-9def-cafef00d1234": true,
		// Wrong version (3):
		"00000000-0000-3000-8000-000000000000": false,
		// Wrong variant (c):
		"00000000-0000-4000-c000-000000000000": false,
		// Wrong length:
		"00000000-0000-4000-8000-00000000000":   false,
		"00000000-0000-4000-8000-0000000000000": false,
		// Uppercase rejected (we standardize on lowercase):
		"00000000-0000-4000-8000-00000000000A": false,
		// Missing hyphens:
		"00000000000040008000000000000000": false,
		// Garbage:
		"not-a-uuid": false,
		"":           false,
	}
	for s, want := range cases {
		got := isValidUUIDv4(s)
		if got != want {
			t.Errorf("isValidUUIDv4(%q) = %v, want %v", s, got, want)
		}
	}
}
